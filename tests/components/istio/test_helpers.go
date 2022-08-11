package istio

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/tidwall/pretty"
	"gitlab.com/rodrigoodhin/gocure/models"
	"gitlab.com/rodrigoodhin/gocure/pkg/gocure"
	"gitlab.com/rodrigoodhin/gocure/report/html"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"

	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	proxyName              = "istio-proxy"
	istioNamespace         = "istio-system"
	evalProfile            = "evaluation"
	prodProfile            = "production"
	deployedKymaProfileVar = "KYMA_PROFILE"
	exportResultVar        = "EXPORT_RESULT"
	cucumberFileName       = "cucumber-report.json"
)

var k8sClient kubernetes.Interface
var dynamicClient dynamic.Interface
var mapper *restmapper.DeferredDiscoveryRESTMapper

//go:embed test/httpbin.yaml
var httpbin []byte

func initK8sClient() (kubernetes.Interface, dynamic.Interface, *restmapper.DeferredDiscoveryRESTMapper) {
	var kubeconfig string
	if kConfig, ok := os.LookupEnv("KUBECONFIG"); !ok {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	} else {
		kubeconfig = kConfig
	}
	_, err := os.Stat(kubeconfig)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("kubeconfig %s does not exist", kubeconfig)
		}
		log.Fatalf(err.Error())
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf(err.Error())
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf(err.Error())
	}
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Fatalf(err.Error())
	}
	dc, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		log.Fatalf(err.Error())
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	return k8sClient, dynClient, mapper
}

func readManifestToUnstructured() ([]unstructured.Unstructured, error) {
	var err error
	var unstructList []unstructured.Unstructured

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(httpbin), 200)
	for {
		var rawObj k8sruntime.RawExtension
		if err = decoder.Decode(&rawObj); err != nil {
			break
		}
		obj, _, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			break
		}
		unstructuredMap, err := k8sruntime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			break
		}
		unstructuredObj := unstructured.Unstructured{Object: unstructuredMap}
		unstructList = append(unstructList, unstructuredObj)
	}
	if err != io.EOF {
		return unstructList, err
	}

	return unstructList, nil
}

func getGroupVersionResource(resource unstructured.Unstructured) schema.GroupVersionResource {
	mapping, err := mapper.RESTMapping(resource.GroupVersionKind().GroupKind(), resource.GroupVersionKind().Version)
	if err != nil {
		log.Fatal(err)
	}
	return mapping.Resource
}

func hasIstioProxy(containers []corev1.Container) bool {
	proxyImage := ""
	for _, container := range containers {
		if container.Name == proxyName {
			proxyImage = container.Image
		}
	}
	return proxyImage != ""
}

func getPodListReport(list *corev1.PodList) string {
	type returnedPodList struct {
		PodList []struct {
			Metadata struct {
				Name              string `json:"name"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	p := returnedPodList{}
	toMarshal, _ := json.Marshal(list)
	_ = json.Unmarshal(toMarshal, &p)
	toPrint, _ := json.Marshal(p)
	return string(pretty.Pretty(toPrint))
}

func listPodsIstioNamespace(istiodPodsSelector metav1.ListOptions) (*corev1.PodList, error) {
	return k8sClient.CoreV1().Pods(istioNamespace).List(context.Background(), istiodPodsSelector)
}

func generateHTMLReport() {
	html := gocure.HTML{
		Config: html.Data{
			InputJsonPath:    cucumberFileName,
			OutputHtmlFolder: "reports/",
			Title:            "Kyma Istio component tests",
			Metadata: models.Metadata{
				TestEnvironment: os.Getenv(deployedKymaProfileVar),
				Platform:        runtime.GOOS,
				Parallel:        "Scenarios",
				Executed:        "Remote",
				AppVersion:      "main",
				Browser:         "default",
			},
		},
	}
	err := html.Generate()
	if err != nil {
		log.Fatalf(err.Error())
	}
}