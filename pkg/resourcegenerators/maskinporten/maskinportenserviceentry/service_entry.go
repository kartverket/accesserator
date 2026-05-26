package maskinportenserviceentry

import (
	"github.com/kartverket/accesserator/internal/state"
	"github.com/kartverket/accesserator/pkg/config"
	"github.com/kartverket/accesserator/pkg/utilities"
	istioapiv1 "istio.io/api/networking/v1"
	istionetworkingv1 "istio.io/client-go/pkg/apis/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IstioGatewayNamesapce = "istio-gateway"
	IstiodNamesapce       = "istio-system"
)

func GetDesired(objectMeta v1.ObjectMeta, maskinportenConfig state.MaskinportenConfig) *istionetworkingv1.ServiceEntry {
	if !maskinportenConfig.Enabled {
		return nil
	}

	var maskinportenHost string
	if *config.Get().RunsInProduction {
		maskinportenHost = utilities.MaskinportenProdHost
	} else {
		maskinportenHost = utilities.MaskinportenTestHost
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
				maskinportenHost,
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
