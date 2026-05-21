package azureadserviceentry

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/utilities"
	istioapiv1 "istio.io/api/networking/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IstioGatewayNamesapce = "istio-gateway"
	IstiodNamesapce       = "istio-system"
)

func GetDesired(objectMeta v1.ObjectMeta, entraIdConfig state.EntraIdConfig) *istionetworkingv1.ServiceEntry {
	if !entraIdConfig.Enabled {
		return nil
	}

	return &istionetworkingv1.ServiceEntry{
		ObjectMeta: objectMeta,
		Spec: istioapiv1.ServiceEntry{
			ExportTo: []string{
				".",
				IstioGatewayNamesapce,
				IstiodNamesapce,
			},
			Hosts: []string{
				utilities.EntraIdHost,
			},
			Ports: []*istioapiv1.ServicePort{
				{
					Name:     "https",
					Number:   443,
					Protocol: "HTTPS",
				},
			},
			Resolution: istioapiv1.ServiceEntry_DNS,
		},
	}
}
