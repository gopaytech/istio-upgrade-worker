package configmap

import (
	"context"

	"github.com/gopaytech/istio-upgrade-worker/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kind       = "ConfigMap"
	apiVersion = "v1"
)

type Service interface {
	Create(ctx context.Context, namespace string, name string, data map[string]string) error
	Get(ctx context.Context, namespace string, configMapName string) (*v1.ConfigMap, error)
	Update(ctx context.Context, namespace string, cm *v1.ConfigMap) error
}

func New(cfg *config.Conf) Service {
	return &svc{
		k8sConfig: cfg,
	}
}

type svc struct {
	k8sConfig *config.Conf
}

func (s *svc) Create(ctx context.Context, namespace string, name string, data map[string]string) error {
	c := s.k8sConfig.K8SClient()
	cm := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
	_, err := c.CoreV1().ConfigMaps(namespace).Create(ctx, &cm, metav1.CreateOptions{})
	return err
}

func (s *svc) Get(ctx context.Context, namespace string, configMapName string) (*v1.ConfigMap, error) {
	c := s.k8sConfig.K8SClient()
	cm, err := c.CoreV1().ConfigMaps(namespace).Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func (s *svc) Update(ctx context.Context, namespace string, cm *v1.ConfigMap) error {
	c := s.k8sConfig.K8SClient()
	_, err := c.CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
	return err
}
