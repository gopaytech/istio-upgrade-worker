package kubernetes

import (
	"context"

	"github.com/gopaytech/istio-upgrade-worker/config"
	"github.com/gopaytech/istio-upgrade-worker/settings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Namespace struct {
	Name string `json:"name"`
}

type NamespaceInterface interface {
	GetIstioNamespaces(ctx context.Context) ([]Namespace, error)
}

type NamespaceService struct {
	kubernetesConfig *config.Kubernetes
	canaryLabel      string
	label            string
}

func NewNamespaceService(kubernetesConfig *config.Kubernetes, settings settings.Settings) NamespaceInterface {
	return &NamespaceService{
		kubernetesConfig: kubernetesConfig,
		canaryLabel:      settings.IstioNamespaceCanaryLabel,
		label:            settings.IstioNamespaceLabel,
	}
}

func (s *NamespaceService) GetIstioNamespaces(ctx context.Context) ([]Namespace, error) {
	canaryNamespaces, err := s.GetNamespaceByLabel(ctx, s.canaryLabel)
	if err != nil {
		return nil, err
	}
	nonCanaryNamespaces, err := s.GetNamespaceByLabel(ctx, s.label)
	if err != nil {
		return nil, err
	}
	var namespaces []Namespace
	namespaces = append(namespaces, canaryNamespaces...)
	namespaces = append(namespaces, nonCanaryNamespaces...)
	return namespaces, nil
}

func (s *NamespaceService) GetNamespaceByLabel(ctx context.Context, label string) ([]Namespace, error) {
	c := s.kubernetesConfig.Client()
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
