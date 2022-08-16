package store

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"kubectlfzf/pkg/k8s/clusterconfig"
	"kubectlfzf/pkg/k8s/resources"
	"kubectlfzf/pkg/util"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	logrus.SetLevel(logrus.DebugLevel)
	code := m.Run()
	os.Exit(code)
}

func podResource(name string, ns string, labels map[string]string) corev1.Pod {
	meta := corev1.Pod{
		TypeMeta: metav1.TypeMeta{Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			Labels:            labels,
			CreationTimestamp: metav1.Time{Time: time.Now()},
		},
		Spec:   corev1.PodSpec{},
		Status: corev1.PodStatus{},
	}
	return meta
}

func getPodK8sStore(t *testing.T) (string, *Store) {
	tempDir, err := ioutil.TempDir("/tmp/", "cacheTest")
	assert.Nil(t, err)

	storeConfigCli := &StoreConfigCli{
		ClusterConfigCli: clusterconfig.ClusterConfigCli{
			ClusterName: "test", CacheDir: tempDir, Kubeconfig: ""},
		TimeBetweenFullDump: 500 * time.Millisecond}
	storeConfig := NewStoreConfig(storeConfigCli)
	err = storeConfig.CreateDestDir()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctorConfig := resources.CtorConfig{Cluster: "clstr"}
	k8sStore := NewStore(ctx, storeConfig, ctorConfig, resources.ResourceTypePod)
	assert.Nil(t, err)

	pods := []corev1.Pod{
		podResource("Test1", "ns1", map[string]string{"app": "app1"}),
		podResource("Test2", "ns2", map[string]string{"app": "app2"}),
		podResource("Test3", "ns2", map[string]string{"app": "app2"}),
		podResource("Test4", "aaa", map[string]string{"app": "app3"}),
	}

	for _, pod := range pods {
		k8sStore.AddResource(&pod)
	}
	return tempDir, k8sStore
}

func TestDumpPodFullState(t *testing.T) {
	tempDir, k := getPodK8sStore(t)
	defer util.RemoveTempDir(tempDir)

	err := k.DumpFullState()
	require.NoError(t, err)
	podFilePath := path.Join(tempDir, "test", "pods")
	assert.FileExists(t, podFilePath)

	pods := map[string]resources.K8sResource{}
	err = util.LoadFromFile(&pods, podFilePath)
	require.NoError(t, err)

	assert.Equal(t, 4, len(pods))
	assert.Contains(t, pods, "ns1_Test1")
	assert.Contains(t, pods, "ns2_Test2")
	assert.Contains(t, pods, "ns2_Test3")
	assert.Contains(t, pods, "aaa_Test4")
}

func TestTickerPodDumpFullState(t *testing.T) {
	tempDir, k := getPodK8sStore(t)
	defer util.RemoveTempDir(tempDir)

	time.Sleep(1000 * time.Millisecond)
	podFilePath := path.Join(tempDir, "test", "pods")
	assert.FileExists(t, podFilePath)
	pods := map[string]resources.K8sResource{}
	err := util.LoadFromFile(&pods, podFilePath)
	require.NoError(t, err)
	assert.Equal(t, 4, len(pods))

	pod := podResource("Test1", "ns1", map[string]string{"app": "app1"})
	k.AddResource(&pod)
	assert.True(t, k.dumpRequired)
	time.Sleep(1000 * time.Millisecond)
	assert.False(t, k.dumpRequired)
}

func TestDumpAPIResources(t *testing.T) {
	resource := map[string]resources.K8sResource{}

	list := resources.APIResourceList{}
	list.GroupVersion = "v1"

	a := resources.APIResource{}
	a.Shortnames = []string{"short"}
	a.Name = "name"
	list.ApiResources = append(list.ApiResources, a)

	resource["v1"] = &list
	tempDir, err := ioutil.TempDir("/tmp/", "cacheTest")
	require.NoError(t, err)

	apiResourcesFilePath := path.Join(tempDir, "apiresources")
	err = util.EncodeToFile(resource, apiResourcesFilePath)
	require.NoError(t, err)

	loadResource := map[string]resources.K8sResource{}
	err = util.LoadFromFile(&loadResource, apiResourcesFilePath)
	require.NoError(t, err)
}
