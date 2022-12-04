package appset

import (
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
)

// AppRBACName formats fully qualified application name for RBAC check
func AppSetRBACName(appSet *v1alpha1.ApplicationSet) (string, error) {

	projectName, err := argo.GetAppSetProject(*appSet)

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", projectName, appSet.ObjectMeta.Name), nil
}
