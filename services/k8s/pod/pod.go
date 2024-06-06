package pod

import (
	"context"

	"github.com/gopaytech/istio-upgrade-worker/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type svc struct {
	k8sConfig *config.Conf
}

type Service interface {
	FindByNamespaceAndLabels(ctx context.Context, namespace string, labels map[string]string) ([]v1.Pod, error)
}

func New(cfg *config.Conf) Service {
	return &svc{
		k8sConfig: cfg,
	}
}

func (s *svc) FindByNamespaceAndLabels(ctx context.Context, namespace string, mapLabels map[string]string) ([]v1.Pod, error) {
	c := s.k8sConfig.K8SClient()
	labelSelector := metav1.LabelSelector{MatchLabels: mapLabels}
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	}
	podList, err := c.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}
	pods := podList.Items
	return pods, nil
}
