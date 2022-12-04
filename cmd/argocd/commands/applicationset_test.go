package commands

import (
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	arogappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPrintApplicationSetNames(t *testing.T) {
	output, _ := captureOutput(func() error {
		appSet := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		printApplicationSetNames([]arogappsetv1.ApplicationSet{*appSet, *appSet})
		return nil
	})
	expectation := "test\ntest\n"
	if output != expectation {
		t.Fatalf("Incorrect print params output %q, should be %q", output, expectation)
	}
}
func TestPrintApplicationSetTable(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: arogappsetv1.ApplicationSetSpec{
				Generators: []arogappsetv1.ApplicationSetGenerator{
					arogappsetv1.ApplicationSetGenerator{
						Git: &arogappsetv1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []arogappsetv1.GitDirectoryGeneratorItem{
								arogappsetv1.GitDirectoryGeneratorItem{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: arogappsetv1.ApplicationSetTemplate{
					Spec: &apiextensionsv1.JSON{Raw: []byte(`{"project": "default"}`)},
				},
			},
			Status: arogappsetv1.ApplicationSetStatus{
				Conditions: []arogappsetv1.ApplicationSetCondition{
					arogappsetv1.ApplicationSetCondition{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   arogappsetv1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}
		output := "table"
		printApplicationSetTable([]arogappsetv1.ApplicationSet{*app, *app}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := `NAME      NAMESPACE  PROJECT  SYNCPOLICY  CONDITIONS
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]
`
	assert.Equal(t, expectation, output)
}

func TestPrintApplicationSetTableWithTemplatedFields(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: arogappsetv1.ApplicationSetSpec{
				Generators: []arogappsetv1.ApplicationSetGenerator{
					arogappsetv1.ApplicationSetGenerator{
						Git: &arogappsetv1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []arogappsetv1.GitDirectoryGeneratorItem{
								arogappsetv1.GitDirectoryGeneratorItem{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: arogappsetv1.ApplicationSetTemplate{
					Spec: &apiextensionsv1.JSON{Raw: []byte(`{"project": "default", "source": {"repoURL": "https://github.com/argoproj/argocd-example-apps", "targetRevision": "{{ .targetRevision }}", "path": "guestbook"}}`)},
				},
			},
			Status: arogappsetv1.ApplicationSetStatus{
				Conditions: []arogappsetv1.ApplicationSetCondition{
					arogappsetv1.ApplicationSetCondition{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   arogappsetv1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}
		output := "wide"
		printApplicationSetTable([]arogappsetv1.ApplicationSet{*app, *app}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := `NAME      NAMESPACE  PROJECT  SYNCPOLICY  CONDITIONS                          REPO                                             PATH       TARGET
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  https://github.com/argoproj/argocd-example-apps  guestbook  {{ .targetRevision }}
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  https://github.com/argoproj/argocd-example-apps  guestbook  {{ .targetRevision }}
`
	assert.Equal(t, expectation, output)
}

func TestSourceTemplated(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: arogappsetv1.ApplicationSetSpec{
				Generators: []arogappsetv1.ApplicationSetGenerator{
					arogappsetv1.ApplicationSetGenerator{
						Git: &arogappsetv1.GitGenerator{
							RepoURL:  "https://github.com/argoproj/argo-cd.git",
							Revision: "head",
							Directories: []arogappsetv1.GitDirectoryGeneratorItem{
								arogappsetv1.GitDirectoryGeneratorItem{
									Path: "applicationset/examples/git-generator-directory/cluster-addons/*",
								},
							},
						},
					},
				},
				Template: arogappsetv1.ApplicationSetTemplate{
					Spec: &apiextensionsv1.JSON{Raw: []byte(`{"project": "default", 
					"{{ ternary \"source\" \"nosourcegit\" (eq .type \"git\") }}": {"repoURL": "https://github.com/argoproj/argocd-example-apps", "targetRevision": "{{ .targetRevision }}", "path": "guestbook"}, 
					"{{ ternary \"source\" \"nosourcehelm\" (eq .type \"helm\") }}": {"repoURL": "https://chart.github.com/argoproj/argocd-example-apps", "targetRevision": "{{ .targetRevision }}", "path": "guestbook"}}`)},
				},
			},
			Status: arogappsetv1.ApplicationSetStatus{
				Conditions: []arogappsetv1.ApplicationSetCondition{
					arogappsetv1.ApplicationSetCondition{
						Status: v1alpha1.ApplicationSetConditionStatusTrue,
						Type:   arogappsetv1.ApplicationSetConditionResourcesUpToDate,
					},
				},
			},
		}
		output := "wide"
		printApplicationSetTable([]arogappsetv1.ApplicationSet{*app, *app}, &output)
		return nil
	})
	assert.NoError(t, err)
	expectation := `NAME      NAMESPACE  PROJECT  SYNCPOLICY  CONDITIONS                          REPO       PATH       TARGET
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  Templated  Templated  Templated
app-name             default  nil         [{ResourcesUpToDate  <nil> True }]  Templated  Templated  Templated
`
	assert.Equal(t, expectation, output)
}

func TestShowAppSetTemplate(t *testing.T) {
	output, _ := captureOutput(func() error {
		appSet := &arogappsetv1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: arogappsetv1.ApplicationSetSpec{
				Template: arogappsetv1.ApplicationSetTemplate{
					ApplicationSetTemplateMeta: arogappsetv1.ApplicationSetTemplateMeta{
						Name: "{{ .name }}",
						Labels: map[string]string{
							"foo": "{{ .bar }}",
						},
					},
					Spec: &apiextensionsv1.JSON{Raw: []byte(`{"project": "default", "source": {"repoURL": "https://github.com/argoproj/argocd-example-apps", "targetRevision": "{{ .targetRevision }}", "path": "guestbook"}}`)},
				},
			},
		}

		var template map[string]interface{}
		err := json.Unmarshal(appSet.Spec.Template.Spec.Raw, &template)

		assert.NoError(t, err)

		showAppSetTemplate(appSet.Spec.Template.ApplicationSetTemplateMeta, template)
		return nil
	})
	expectation := `Template:
Metadata:
labels:
  foo: '{{ .bar }}'
name: '{{ .name }}'
Spec:
project: default
source:
  path: guestbook
  repoURL: https://github.com/argoproj/argocd-example-apps
  targetRevision: '{{ .targetRevision }}'
`
	assert.Equal(t, expectation, output)
}
