package g8scontrolplane

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	infrastructurev1alpha2 "github.com/giantswarm/apiextensions/pkg/apis/infrastructure/v1alpha2"
	releasev1alpha1 "github.com/giantswarm/apiextensions/pkg/apis/release/v1alpha1"
	"github.com/giantswarm/backoff"
	"github.com/giantswarm/k8sclient/v3/pkg/k8sclient"
	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	restclient "k8s.io/client-go/rest"
	apiv1alpha2 "sigs.k8s.io/cluster-api/api/v1alpha2"

	"github.com/giantswarm/admission-controller/pkg/admission"
)

type Admitter struct {
	k8sClient              k8sclient.Interface
	validAvailabilityZones []string
}

type AdmitterConfig struct {
	ValidAvailabilityZones string
}

// var (
//  g8sControlPlaneResource = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "g8scontrolplane"}
// )

func NewAdmitter(cfg *AdmitterConfig) (*Admitter, error) {
	var k8sClient k8sclient.Interface
	{
		restConfig, err := restclient.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load key kubeconfig: %v", err)
		}
		newLogger, err := micrologger.New(micrologger.Config{})
		if err != nil {
			return nil, err
		}
		c := k8sclient.ClientsConfig{
			SchemeBuilder: k8sclient.SchemeBuilder{
				apiv1alpha2.AddToScheme,
				infrastructurev1alpha2.AddToScheme,
				releasev1alpha1.AddToScheme,
			},
			Logger: newLogger,

			RestConfig: restConfig,
		}

		k8sClient, err = k8sclient.NewClients(c)
		if err != nil {
			return nil, microerror.Mask(err)
		}
	}

	var availabilityZones []string = strings.Split(cfg.ValidAvailabilityZones, ",")

	admitter := &Admitter{
		k8sClient:              k8sClient,
		validAvailabilityZones: availabilityZones,
	}

	log.Infof("Valid Availavility Zones %v", availabilityZones)

	return admitter, nil
}

func (admitter *Admitter) Admit(request *v1beta1.AdmissionRequest) ([]admission.PatchOperation, error) {
	g8sControlPlaneNewCR := &infrastructurev1alpha2.G8sControlPlane{}
	g8sControlPlaneOldCR := &infrastructurev1alpha2.G8sControlPlane{}
	if _, _, err := admission.Deserializer.Decode(request.Object.Raw, nil, g8sControlPlaneNewCR); err != nil {
		log.Errorf("unable to parse g8scontrol plane: %v", err)
		return nil, admission.InternalError
	}
	if _, _, err := admission.Deserializer.Decode(request.OldObject.Raw, nil, g8sControlPlaneOldCR); err != nil {
		log.Errorf("unable to parse g8scontrol plane: %v", err)
		return nil, admission.InternalError
	}

	var result []admission.PatchOperation
	// Trigger master upgrade from single to HA
	if g8sControlPlaneNewCR.Spec.Replicas == 3 && g8sControlPlaneOldCR.Spec.Replicas == 1 {
		update := func() error {

			ctx := context.Background()

			// We fetch the AWSControlPlane CR.
			awsControlPlane := &infrastructurev1alpha2.AWSControlPlane{}
			{
				log.Infof("Fetching AWSControlPlane %s", g8sControlPlaneNewCR.Name)
				err := admitter.k8sClient.CtrlClient().Get(ctx,
					types.NamespacedName{Name: g8sControlPlaneNewCR.GetName(),
						Namespace: g8sControlPlaneNewCR.GetNamespace()},
					awsControlPlane)
				if err != nil {
					return fmt.Errorf("failed to fetch AWSControlplane: %v", err)
				}
			}

			// If the availability zones need to be updated from 1 to 3, we do it here
			{
				if len(awsControlPlane.Spec.AvailabilityZones) == 1 {
					log.Infof("Updating AWSControlPlane AZs for HA %s", awsControlPlane.Name)
					awsControlPlane.Spec.AvailabilityZones = getHAavailabilityZones(awsControlPlane.Spec.AvailabilityZones[0], admitter.validAvailabilityZones)
					err := admitter.k8sClient.CtrlClient().Update(ctx, awsControlPlane)
					if err != nil {
						return fmt.Errorf("failed to update AWSControlplane: %v", err)
					}
				}
				return nil
			}
		}
		b := backoff.NewMaxRetries(3, 1*time.Second)
		err := backoff.Retry(update, b)
		if err != nil {
			log.Errorf("Failed to update AWSControlPlane to 3 master replicas: %v", err)
			return nil, admission.InternalError
		}

	}
	return result, nil
}

func getHAavailabilityZones(firstAZ string, azs []string) []string {
	var randomAZs []string
	// Having 3 AZ's or more shuffle 3 HA masters in different AZ's
	if len(azs) >= 3 {
		var tempAZs []string
		for _, az := range azs {
			if firstAZ != az {
				tempAZs = append(tempAZs, az)
			}
		}
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(tempAZs), func(i, j int) { tempAZs[i], tempAZs[j] = tempAZs[j], tempAZs[i] })
		randomAZs = append(randomAZs, firstAZ, tempAZs[0], tempAZs[1])
		log.Infof("%d AZ's available, selected AZ's: %v", len(azs), randomAZs)

		return randomAZs

		// Having only 2 AZ available we shuffle 3 HA masters in 2 AZ's
	} else if len(azs) == 2 {
		var tempAZ string
		for _, az := range azs {
			if firstAZ != az {
				tempAZ = az
			}
		}
		rand.Seed(time.Now().UnixNano())
		rand.Shuffle(len(azs), func(i, j int) { azs[i], azs[j] = azs[j], azs[i] })
		randomAZs = append(randomAZs, firstAZ, tempAZ, azs[0])
		log.Infof("Only %d AZ's available, random AZ's will be %v", len(azs), randomAZs)

		return randomAZs

		// Having only 1 AZ available we add 3 HA masters to this AZ
	} else {
		randomAZs = append(randomAZs, firstAZ, firstAZ, firstAZ)
		log.Infof("Only %d AZ's available, using the same AZ %v", len(azs), randomAZs)

		return randomAZs
	}
}
