package jindofsx

import (
	"github.com/fluid-cloudnative/fluid/pkg/common"
	"reflect"
	"testing"

	"github.com/fluid-cloudnative/fluid/pkg/utils/fake"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/agiledragon/gomonkey"
	datav1alpha1 "github.com/fluid-cloudnative/fluid/api/v1alpha1"
	"github.com/fluid-cloudnative/fluid/pkg/utils"
	. "github.com/smartystreets/goconvey/convey"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestQueryCacheStatus(t *testing.T) {
	Convey("test queryCacheStatus ", t, func() {
		Convey("with dataset UFSTotal is not empty ", func() {
			var engine *JindoFSxEngine
			patch1 := ApplyMethod(reflect.TypeOf(engine), "GetReportSummary",
				func(_ *JindoFSxEngine) (string, error) {
					summary := mockJindoReportSummary()
					return summary, nil
				})
			defer patch1.Reset()

			patch2 := ApplyFunc(utils.GetDataset,
				func(_ client.Client, _ string, _ string) (*datav1alpha1.Dataset, error) {
					d := &datav1alpha1.Dataset{
						Status: datav1alpha1.DatasetStatus{
							UfsTotal: "52.18MiB",
						},
					}
					return d, nil
				})
			defer patch2.Reset()

			e := &JindoFSxEngine{
				runtime: &datav1alpha1.JindoRuntime{Spec: datav1alpha1.JindoRuntimeSpec{
					TieredStore: datav1alpha1.TieredStore{
						Levels: []datav1alpha1.Level{
							{
								Path:       "/mnt/jindo0",
								MediumType: common.HDD,
							},
						},
					}},
				},
			}
			got, err := e.queryCacheStatus()
			want := cacheStates{
				cacheCapacity:    "250.38GiB",
				cached:           "11.72GiB",
				cachedPercentage: "100.0%",
			}

			So(got, ShouldResemble, want)
			So(err, ShouldEqual, nil)
		})

		Convey("with dataset UFSTotal is: [Calculating]", func() {
			var engine *JindoFSxEngine
			patch1 := ApplyMethod(reflect.TypeOf(engine), "GetReportSummary",
				func(_ *JindoFSxEngine) (string, error) {
					summary := mockJindoReportSummary()
					return summary, nil
				})
			defer patch1.Reset()

			patch2 := ApplyFunc(utils.GetDataset,
				func(_ client.Client, _ string, _ string) (*datav1alpha1.Dataset, error) {
					d := &datav1alpha1.Dataset{
						Status: datav1alpha1.DatasetStatus{
							UfsTotal: "[Calculating]",
						},
					}
					return d, nil
				})
			defer patch2.Reset()

			e := &JindoFSxEngine{
				runtime: &datav1alpha1.JindoRuntime{Spec: datav1alpha1.JindoRuntimeSpec{
					TieredStore: datav1alpha1.TieredStore{
						Levels: []datav1alpha1.Level{
							{
								Path:       "/mnt/jindo0",
								MediumType: common.HDD,
							},
						},
					}},
				},
			}
			got, err := e.queryCacheStatus()
			want := cacheStates{
				cacheCapacity: "250.38GiB",
				cached:        "11.72GiB",
			}

			So(got, ShouldResemble, want)
			So(err, ShouldEqual, nil)
		})

		Convey("with dataset UFSTotal is empty", func() {
			var engine *JindoFSxEngine
			patch1 := ApplyMethod(reflect.TypeOf(engine), "GetReportSummary",
				func(_ *JindoFSxEngine) (string, error) {
					summary := mockJindoReportSummary()
					return summary, nil
				})
			defer patch1.Reset()

			patch2 := ApplyFunc(utils.GetDataset,
				func(_ client.Client, _ string, _ string) (*datav1alpha1.Dataset, error) {
					d := &datav1alpha1.Dataset{
						Status: datav1alpha1.DatasetStatus{
							UfsTotal: "",
						},
					}
					return d, nil
				})
			defer patch2.Reset()

			e := &JindoFSxEngine{
				runtime: &datav1alpha1.JindoRuntime{Spec: datav1alpha1.JindoRuntimeSpec{
					TieredStore: datav1alpha1.TieredStore{
						Levels: []datav1alpha1.Level{
							{
								Path:       "/mnt/jindo0",
								MediumType: common.HDD,
							},
						},
					}},
				},
			}
			got, err := e.queryCacheStatus()
			want := cacheStates{
				cacheCapacity: "250.38GiB",
				cached:        "11.72GiB",
			}

			So(got, ShouldResemble, want)
			So(err, ShouldEqual, nil)
		})
	})
}

func TestInvokeCleanCache(t *testing.T) {
	masterInputs := []*appsv1.StatefulSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hadoop-jindofs-master",
				Namespace: "fluid",
			},
			Status: appsv1.StatefulSetStatus{
				ReadyReplicas: 0,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "hbase-jindofs-master",
				Namespace: "fluid",
			},
			Status: appsv1.StatefulSetStatus{
				ReadyReplicas: 1,
			},
		},
	}
	objs := []runtime.Object{}
	for _, masterInput := range masterInputs {
		objs = append(objs, masterInput.DeepCopy())
	}
	fakeClient := fake.NewFakeClientWithScheme(testScheme, objs...)
	testCases := []struct {
		name      string
		namespace string
		isErr     bool
	}{
		{
			name:      "hadoop",
			namespace: "fluid",
			isErr:     false,
		},
		{
			name:      "hbase",
			namespace: "fluid",
			isErr:     true,
		},
		{
			name:      "none",
			namespace: "fluid",
			isErr:     false,
		},
	}
	for _, testCase := range testCases {
		engine := &JindoFSxEngine{
			Client:    fakeClient,
			namespace: testCase.namespace,
			name:      testCase.name,
			Log:       fake.NullLogger(),
		}
		err := engine.invokeCleanCache()
		isErr := err != nil
		if isErr != testCase.isErr {
			t.Errorf("test-name:%s want %t, got %t", testCase.name, testCase.isErr, isErr)
		}
	}
}

//
// $ jindo fs -report
//
func mockJindoReportSummary() string {
	s := `Namespace Address: localhost:18000
	Rpc Port: 8101
	Started: Mon Jul 19 07:41:39 2021
	Version: 3.6.1
	Live Nodes: 2
	Decommission Nodes: 0
	Mode: BLOCK
	Total Disk Capacity: 250.38GB
	Used Disk Capacity: 11.72GB
    Total MEM Capacity: 250.38GB
	Used MEM Capacity: 11.72GB
	`
	return s
}
