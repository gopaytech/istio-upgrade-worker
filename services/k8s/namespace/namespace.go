package namespace

import (
	"context"

	"github.com/gopaytech/istio-upgrade-worker/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	canaryLabel    = "istio.io/rev=default"
	nonCanaryLabel = "istio-injection=enabled"
)

type Namespace struct {
	Name string `json:"name"`
}

type Service interface {
	GetIstioNamespaces(ctx context.Context) ([]Namespace, error)
}

type svc struct {
	k8sConfig *config.Conf
}

func New(cfg *config.Conf) Service {
	return &svc{
		k8sConfig: cfg,
	}
}

func (s *svc) GetIstioNamespaces(ctx context.Context) ([]Namespace, error) {

	canaryNamespaces, err := s.GetNamespaceByLabel(ctx, canaryLabel)
	if err != nil {
		return nil, err
	}
	nonCanaryNamespaces, err := s.GetNamespaceByLabel(ctx, nonCanaryLabel)
	if err != nil {
		return nil, err
	}
	var namespaces []Namespace
	namespaces = append(namespaces, canaryNamespaces...)
	namespaces = append(namespaces, nonCanaryNamespaces...)
	return namespaces, nil
}

func (s *svc) GetNamespaceByLabel(ctx context.Context, label string) ([]Namespace, error) {
	c := s.k8sConfig.K8SClient()
	nsList, err := c.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: label,
	})
	if err != nil {
		return nil, err
	}
	items := nsList.Items
	var namespaces []Namespace
	for _, ns := range items {
		namespace := Namespace{Name: ns.Name}
		namespaces = append(namespaces, namespace)
	}
	return namespaces, nil
}
