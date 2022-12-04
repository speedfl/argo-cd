package utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	argoappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func initFieldMapTemplate(value string) map[string]apiextensionsv1.JSON {

	value = strings.ReplaceAll(value, `"`, `\"`)
	fieldMap := map[string]apiextensionsv1.JSON{}
	fieldMap["Path"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"source":{"path": "%s"}}`, value))}
	fieldMap["RepoURL"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"source":{"repoURL": "%s"}}`, value))}
	fieldMap["TargetRevision"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"source":{"targetRevision": "%s"}}`, value))}
	fieldMap["Chart"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"source":{"chart": "%s"}}`, value))}

	fieldMap["Server"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"destination":{"server": "%s"}}`, value))}
	fieldMap["Namespace"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"destination":{"namespace": "%s"}}`, value))}
	fieldMap["Name"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"destination":{"name": "%s"}}`, value))}
	fieldMap["Project"] = apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{"project": "%s"}`, value))}

	return fieldMap
}

func initFieldMapApplication() map[string]func(app *argoappsv1.Application) *string {

	fieldMap := map[string]func(app *argoappsv1.Application) *string{}
	fieldMap["Path"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.Path }
	fieldMap["RepoURL"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.RepoURL }
	fieldMap["TargetRevision"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.TargetRevision }
	fieldMap["Chart"] = func(app *argoappsv1.Application) *string { return &app.Spec.Source.Chart }
	fieldMap["Server"] = func(app *argoappsv1.Application) *string { return &app.Spec.Destination.Server }
	fieldMap["Namespace"] = func(app *argoappsv1.Application) *string { return &app.Spec.Destination.Namespace }
	fieldMap["Name"] = func(app *argoappsv1.Application) *string { return &app.Spec.Destination.Name }
	fieldMap["Project"] = func(app *argoappsv1.Application) *string { return &app.Spec.Project }
	return fieldMap
}

func TestRenderTemplateParams(t *testing.T) {

	// Believe it or not, this is actually less complex than the equivalent solution using reflection
	fieldMap := initFieldMapApplication()

	emptyApplication := &argoappsv1.ApplicationSetTemplate{
		ApplicationSetTemplateMeta: argoappsetv1.ApplicationSetTemplateMeta{
			Annotations: map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			Labels:      map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			Name:        "application-one",
			Namespace:   "default",
		},
		Spec: &apiextensionsv1.JSON{Raw: []byte{}},
	}

	tests := []struct {
		name        string
		fieldVal    string
		params      map[string]interface{}
		expectedVal string
	}{
		{
			name:        "simple substitution",
			fieldVal:    "{{one}}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "simple substitution with whitespace",
			fieldVal:    "{{ one }}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},

		{
			name:        "template characters but not in a template",
			fieldVal:    "}} {{",
			expectedVal: "}} {{",
			params: map[string]interface{}{
				"one": "two",
			},
		},

		{
			name:        "nested template",
			fieldVal:    "{{ }}",
			expectedVal: "{{ }}",
			params: map[string]interface{}{
				"one": "{{ }}",
			},
		},
		{
			name:        "field with whitespace",
			fieldVal:    "{{ }}",
			expectedVal: "{{ }}",
			params: map[string]interface{}{
				" ": "two",
				"":  "three",
			},
		},

		{
			name:        "template contains itself, containing itself",
			fieldVal:    "{{one}}",
			expectedVal: "{{one}}",
			params: map[string]interface{}{
				"{{one}}": "{{one}}",
			},
		},

		{
			name:        "template contains itself, containing something else",
			fieldVal:    "{{one}}",
			expectedVal: "{{one}}",
			params: map[string]interface{}{
				"{{one}}": "{{two}}",
			},
		},

		{
			name:        "templates are case sensitive",
			fieldVal:    "{{ONE}}",
			expectedVal: "{{ONE}}",
			params: map[string]interface{}{
				"{{one}}": "two",
			},
		},
		{
			name:        "multiple on a line",
			fieldVal:    "{{one}}{{one}}",
			expectedVal: "twotwo",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "multiple different on a line",
			fieldVal:    "{{one}}{{three}}",
			expectedVal: "twofour",
			params: map[string]interface{}{
				"one":   "two",
				"three": "four",
			},
		},
		{
			name:        "multiple different on a line with quote",
			fieldVal:    "{{one}} {{three}}",
			expectedVal: "\"hello\"\" \\ world four",
			params: map[string]interface{}{
				"one":   "\"hello\"\" \\ world",
				"three": "four",
			},
		},
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {

			fieldMapTemplate := initFieldMapTemplate(test.fieldVal)

			for fieldName, getPtrFunc := range fieldMap {

				// Clone the template application
				application := emptyApplication.DeepCopy()

				// Set the value of the target field, to the test value
				*application.Spec = fieldMapTemplate[fieldName]

				// Render the cloned application, into a new application
				render := Render{}
				newApplication, err := render.RenderTemplateParams(application, nil, test.params, false)

				// Retrieve the value of the target field from the newApplication, then verify that
				// the target field has been templated into the expected value
				actualValue := *getPtrFunc(newApplication)
				assert.Equal(t, test.expectedVal, actualValue, "Field '%s' had an unexpected value. expected: '%s' value: '%s'", fieldName, test.expectedVal, actualValue)
				assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key"], "annotation-value")
				assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key2"], "annotation-value2")
				assert.Equal(t, newApplication.ObjectMeta.Labels["label-key"], "label-value")
				assert.Equal(t, newApplication.ObjectMeta.Labels["label-key2"], "label-value2")
				assert.Equal(t, newApplication.ObjectMeta.Name, "application-one")
				assert.Equal(t, newApplication.ObjectMeta.Namespace, "default")
				assert.NoError(t, err)
			}
		})
	}

}

func TestRenderTemplateParamsGoTemplate(t *testing.T) {

	// Believe it or not, this is actually less complex than the equivalent solution using reflection
	fieldMap := initFieldMapApplication()

	emptyApplication := &argoappsv1.ApplicationSetTemplate{
		ApplicationSetTemplateMeta: argoappsetv1.ApplicationSetTemplateMeta{
			Annotations: map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			Labels:      map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			Name:        "application-one",
			Namespace:   "default",
		},
		Spec: &apiextensionsv1.JSON{Raw: []byte{}},
	}

	tests := []struct {
		name         string
		fieldVal     string
		params       map[string]interface{}
		expectedVal  string
		errorMessage string
	}{
		{
			name:        "simple substitution",
			fieldVal:    "{{ .one }}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "simple substitution with whitespace",
			fieldVal:    "{{ .one }}",
			expectedVal: "two",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "template contains itself, containing itself",
			fieldVal:    "{{ .one }}",
			expectedVal: "{{one}}",
			params: map[string]interface{}{
				"one": "{{one}}",
			},
		},

		{
			name:        "template contains itself, containing something else",
			fieldVal:    "{{ .one }}",
			expectedVal: "{{two}}",
			params: map[string]interface{}{
				"one": "{{two}}",
			},
		},
		{
			name:        "multiple on a line",
			fieldVal:    "{{.one}}{{.one}}",
			expectedVal: "twotwo",
			params: map[string]interface{}{
				"one": "two",
			},
		},
		{
			name:        "multiple different on a line",
			fieldVal:    "{{.one}}{{.three}}",
			expectedVal: "twofour",
			params: map[string]interface{}{
				"one":   "two",
				"three": "four",
			},
		},
		{
			name:        "multiple different on a line with quote",
			fieldVal:    "{{.one}} {{.three}}",
			expectedVal: "\"hello\" world four",
			params: map[string]interface{}{
				"one":   "\"hello\" world",
				"three": "four",
			},
		},
		{
			name:        "depth",
			fieldVal:    "{{ .image.version }}",
			expectedVal: "latest",
			params: map[string]interface{}{
				"replicas": 3,
				"image": map[string]interface{}{
					"name":    "busybox",
					"version": "latest",
				},
			},
		},
		{
			name:        "multiple depth",
			fieldVal:    "{{ .image.name }}:{{ .image.version }}",
			expectedVal: "busybox:latest",
			params: map[string]interface{}{
				"replicas": 3,
				"image": map[string]interface{}{
					"name":    "busybox",
					"version": "latest",
				},
			},
		},
		{
			name:        "if ok",
			fieldVal:    "{{ if .hpa.enabled }}{{ .hpa.maxReplicas }}{{ else }}{{ .replicas }}{{ end }}",
			expectedVal: "5",
			params: map[string]interface{}{
				"replicas": 3,
				"hpa": map[string]interface{}{
					"enabled":     true,
					"minReplicas": 1,
					"maxReplicas": 5,
				},
			},
		},
		{
			name:        "if not ok",
			fieldVal:    "{{ if .hpa.enabled }}{{ .hpa.maxReplicas }}{{ else }}{{ .replicas }}{{ end }}",
			expectedVal: "3",
			params: map[string]interface{}{
				"replicas": 3,
				"hpa": map[string]interface{}{
					"enabled":     false,
					"minReplicas": 1,
					"maxReplicas": 5,
				},
			},
		},
		{
			name:        "loop",
			fieldVal:    "{{ range .volumes }}[{{ .name }}]{{ end }}",
			expectedVal: "[volume-one][volume-two]",
			params: map[string]interface{}{
				"replicas": 3,
				"volumes": []map[string]interface{}{
					{
						"name":     "volume-one",
						"emptyDir": map[string]interface{}{},
					},
					{
						"name":     "volume-two",
						"emptyDir": map[string]interface{}{},
					},
				},
			},
		},
		{
			name:        "Index",
			fieldVal:    `{{ index .admin "admin-ca" }}, {{ index .admin "admin-jks" }}`,
			expectedVal: "value admin ca, value admin jks",
			params: map[string]interface{}{
				"admin": map[string]interface{}{
					"admin-ca":  "value admin ca",
					"admin-jks": "value admin jks",
				},
			},
		},
		{
			name:        "Index",
			fieldVal:    `{{ index .admin "admin-ca" }}, \\ "Hello world", {{ index .admin "admin-jks" }}`,
			expectedVal: `value "admin" ca with \, \ "Hello world", value admin jks`,
			params: map[string]interface{}{
				"admin": map[string]interface{}{
					"admin-ca":  `value "admin" ca with \`,
					"admin-jks": "value admin jks",
				},
			},
		},
		{
			name:        "quote",
			fieldVal:    `{{.quote}}`,
			expectedVal: `"`,
			params: map[string]interface{}{
				"quote": `"`,
			},
		},
		{
			name:        "Test No Data",
			fieldVal:    `{{.data}}`,
			expectedVal: "{{.data}}",
			params:      map[string]interface{}{},
		},
		{
			name:        "Test Parse Error",
			fieldVal:    `{{functiondoesnotexist}}`,
			expectedVal: "",
			params: map[string]interface{}{
				"data": `a data string`,
			},
			errorMessage: `function "functiondoesnotexist" not defined`,
		},
		{
			name:        "Test template error",
			fieldVal:    `{{.data.test}}`,
			expectedVal: "",
			params: map[string]interface{}{
				"data": `a data string`,
			},
			errorMessage: `at <.data.test>: can't evaluate field test in type interface {}`,
		},
	}

	for _, test := range tests {

		fieldMapTemplate := initFieldMapTemplate(test.fieldVal)

		t.Run(test.name, func(t *testing.T) {

			for fieldName, getPtrFunc := range fieldMap {

				// Clone the template application
				application := emptyApplication.DeepCopy()

				// Set the value of the target field, to the test value
				*application.Spec = fieldMapTemplate[fieldName]

				// Render the cloned application, into a new application
				render := Render{}
				newApplication, err := render.RenderTemplateParams(application, nil, test.params, true)

				// Retrieve the value of the target field from the newApplication, then verify that
				// the target field has been templated into the expected value
				if test.errorMessage != "" {
					assert.Error(t, err)
					assert.True(t, strings.Contains(err.Error(), test.errorMessage))
				} else {
					assert.NoError(t, err)
					actualValue := *getPtrFunc(newApplication)
					assert.Equal(t, test.expectedVal, actualValue, "Field '%s' had an unexpected value. expected: '%s' value: '%s'", fieldName, test.expectedVal, actualValue)
					assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key"], "annotation-value")
					assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-key2"], "annotation-value2")
					assert.Equal(t, newApplication.ObjectMeta.Labels["label-key"], "label-value")
					assert.Equal(t, newApplication.ObjectMeta.Labels["label-key2"], "label-value2")
					assert.Equal(t, newApplication.ObjectMeta.Name, "application-one")
					assert.Equal(t, newApplication.ObjectMeta.Namespace, "default")
				}
			}
		})
	}
}

func TestRenderMetadata(t *testing.T) {
	t.Run("fasttemplate", func(t *testing.T) {
		application := &argoappsv1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: argoappsetv1.ApplicationSetTemplateMeta{
				Annotations: map[string]string{
					"annotation-{{key}}": "annotation-{{value}}",
				},
			},
			Spec: &apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{}`))},
		}

		params := map[string]interface{}{
			"key":   "some-key",
			"value": "some-value",
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, false)
		require.NoError(t, err)
		require.Contains(t, newApplication.ObjectMeta.Annotations, "annotation-some-key")
		assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-some-key"], "annotation-some-value")
	})
	t.Run("gotemplate", func(t *testing.T) {
		application := &argoappsv1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: argoappsetv1.ApplicationSetTemplateMeta{
				Annotations: map[string]string{
					"annotation-{{ .key }}": "annotation-{{ .value }}",
				},
			},
			Spec: &apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{}`))},
		}

		params := map[string]interface{}{
			"key":   "some-key",
			"value": "some-value",
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, true)
		require.NoError(t, err)
		require.Contains(t, newApplication.ObjectMeta.Annotations, "annotation-some-key")
		assert.Equal(t, newApplication.ObjectMeta.Annotations["annotation-some-key"], "annotation-some-value")
	})
}

func TestRenderTemplateKeys(t *testing.T) {
	t.Run("automatedPrune", func(t *testing.T) {
		application := &argoappsv1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: argoappsetv1.ApplicationSetTemplateMeta{
				Annotations: map[string]string{},
				Labels:      map[string]string{},
				Name:        "name",
				Namespace:   "namespace",
			},
			Spec: &apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{
				"syncPolicy": {
					"{{ ternary \"automated\" \"notautomated\" .sync.automated}}": {
						"{{ ternary \"prune\" \"notprune\" .sync.prune}}": true
					}
				}

			}`))},
		}

		params := map[string]interface{}{
			"sync": map[string]interface{}{
				"automated": true,
				"prune":     true,
			},
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, true)
		require.NoError(t, err)
		require.True(t, newApplication.Spec.SyncPolicy.Automated.Prune)
	})
	t.Run("automatedNoPrune", func(t *testing.T) {
		application := &argoappsv1.ApplicationSetTemplate{
			ApplicationSetTemplateMeta: argoappsetv1.ApplicationSetTemplateMeta{
				Annotations: map[string]string{},
				Labels:      map[string]string{},
				Name:        "name",
				Namespace:   "namespace",
			},
			Spec: &apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`{
				"syncPolicy": {
					"{{ ternary \"automated\" \"notautomated\" .sync.automated}}": {
						"{{ ternary \"prune\" \"notprune\" .sync.prune}}": true
					}
				}

			}`))},
		}

		params := map[string]interface{}{
			"sync": map[string]interface{}{
				"automated": true,
				"prune":     false,
			},
		}

		render := Render{}
		newApplication, err := render.RenderTemplateParams(application, nil, params, true)
		require.NoError(t, err)
		require.False(t, newApplication.Spec.SyncPolicy.Automated.Prune)
	})
}

func TestRenderTemplateParamsFinalizers(t *testing.T) {

	emptyApplication := &argoappsv1.ApplicationSetTemplate{
		ApplicationSetTemplateMeta: argoappsetv1.ApplicationSetTemplateMeta{
			Annotations: map[string]string{"annotation-key": "annotation-value", "annotation-key2": "annotation-value2"},
			Labels:      map[string]string{"label-key": "label-value", "label-key2": "label-value2"},
			Name:        "application-one",
			Namespace:   "default",
		},
		Spec: &apiextensionsv1.JSON{Raw: []byte("{}")},
	}

	for _, c := range []struct {
		testName           string
		syncPolicy         *argoappsetv1.ApplicationSetSyncPolicy
		existingFinalizers []string
		expectedFinalizers []string
	}{
		{
			testName:           "existing finalizer should be preserved",
			existingFinalizers: []string{"existing-finalizer"},
			syncPolicy:         nil,
			expectedFinalizers: []string{"existing-finalizer"},
		},
		{
			testName:           "background finalizer should be preserved",
			existingFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},

		{
			testName:           "empty finalizer and empty sync should use standard finalizer",
			existingFinalizers: nil,
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},

		{
			testName:           "standard finalizer should be preserved",
			existingFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		{
			testName:           "empty array finalizers should use standard finalizer",
			existingFinalizers: []string{},
			syncPolicy:         nil,
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		{
			testName:           "non-nil sync policy should use standard finalizer",
			existingFinalizers: nil,
			syncPolicy:         &argoappsetv1.ApplicationSetSyncPolicy{},
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io"},
		},
		{
			testName:           "preserveResourcesOnDeletion should not have a finalizer",
			existingFinalizers: nil,
			syncPolicy: &argoappsetv1.ApplicationSetSyncPolicy{
				PreserveResourcesOnDeletion: true,
			},
			expectedFinalizers: nil,
		},
		{
			testName:           "user-specified finalizer should overwrite preserveResourcesOnDeletion",
			existingFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
			syncPolicy: &argoappsetv1.ApplicationSetSyncPolicy{
				PreserveResourcesOnDeletion: true,
			},
			expectedFinalizers: []string{"resources-finalizer.argocd.argoproj.io/background"},
		},
	} {

		t.Run(c.testName, func(t *testing.T) {

			// Clone the template application
			application := emptyApplication.DeepCopy()
			application.Finalizers = c.existingFinalizers

			params := map[string]interface{}{
				"one": "two",
			}

			// Render the cloned application, into a new application
			render := Render{}

			res, err := render.RenderTemplateParams(application, c.syncPolicy, params, true)
			assert.Nil(t, err)

			assert.ElementsMatch(t, res.Finalizers, c.expectedFinalizers)

		})

	}

}

func TestCheckInvalidGenerators(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argoappsetv1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argoappsv1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		testName    string
		appSet      argoappsetv1.ApplicationSet
		expectedMsg string
	}{
		{
			testName: "invalid generator, without annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-set",
					Namespace: "namespace",
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     &argoappsetv1.ListGenerator{},
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
					},
				},
			},
			expectedMsg: "ApplicationSet test-app-set contains unrecognized generators",
		},
		{
			testName: "invalid generator, with annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-set",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{"list":{}},
									{"bbb":{}},
									{"git":{}},
									{"aaa":{}}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     &argoappsetv1.ListGenerator{},
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedMsg: "ApplicationSet test-app-set contains unrecognized generators: aaa, bbb",
		},
	} {
		oldhooks := logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})
		defer logrus.StandardLogger().ReplaceHooks(oldhooks)
		hook := logtest.NewGlobal()

		_ = CheckInvalidGenerators(&c.appSet)
		assert.True(t, len(hook.Entries) >= 1, c.testName)
		assert.NotNil(t, hook.LastEntry(), c.testName)
		if hook.LastEntry() != nil {
			assert.Equal(t, logrus.WarnLevel, hook.LastEntry().Level, c.testName)
			assert.Equal(t, c.expectedMsg, hook.LastEntry().Message, c.testName)
		}
		hook.Reset()
	}
}

func TestInvalidGenerators(t *testing.T) {

	scheme := runtime.NewScheme()
	err := argoappsetv1.AddToScheme(scheme)
	assert.Nil(t, err)
	err = argoappsv1.AddToScheme(scheme)
	assert.Nil(t, err)

	for _, c := range []struct {
		testName        string
		appSet          argoappsetv1.ApplicationSet
		expectedInvalid bool
		expectedNames   map[string]bool
	}{
		{
			testName: "valid generators, with annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{"list":{}},
									{"cluster":{}},
									{"git":{}}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     &argoappsetv1.ListGenerator{},
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: &argoappsetv1.ClusterGenerator{},
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
					},
				},
			},
			expectedInvalid: false,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generators, no annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "valid and invalid generators, no annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: &argoappsetv1.ClusterGenerator{},
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "valid and invalid generators, with annotation",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{"cluster":{}},
									{"bbb":{}},
									{"git":{}},
									{"aaa":{}}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: &argoappsetv1.ClusterGenerator{},
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      &argoappsetv1.GitGenerator{},
						},
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames: map[string]bool{
				"aaa": true,
				"bbb": true,
			},
		},
		{
			testName: "invalid generator, annotation with missing spec",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generator, annotation with missing generators array",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generator, annotation with empty generators array",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
		{
			testName: "invalid generator, annotation with empty generator",
			appSet: argoappsetv1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
					Annotations: map[string]string{
						"kubectl.kubernetes.io/last-applied-configuration": `{
							"spec":{
								"generators":[
									{}
								]
							}
						}`,
					},
				},
				Spec: argoappsetv1.ApplicationSetSpec{
					Generators: []argoappsetv1.ApplicationSetGenerator{
						{
							List:     nil,
							Clusters: nil,
							Git:      nil,
						},
					},
				},
			},
			expectedInvalid: true,
			expectedNames:   map[string]bool{},
		},
	} {
		hasInvalid, names := invalidGenerators(&c.appSet)
		assert.Equal(t, c.expectedInvalid, hasInvalid, c.testName)
		assert.Equal(t, c.expectedNames, names, c.testName)
	}
}

func TestNormalizeBitbucketBasePath(t *testing.T) {
	for _, c := range []struct {
		testName         string
		basePath         string
		expectedBasePath string
	}{
		{
			testName:         "default api url",
			basePath:         "https://company.bitbucket.com",
			expectedBasePath: "https://company.bitbucket.com/rest",
		},
		{
			testName:         "with /rest suffix",
			basePath:         "https://company.bitbucket.com/rest",
			expectedBasePath: "https://company.bitbucket.com/rest",
		},
		{
			testName:         "with /rest/ suffix",
			basePath:         "https://company.bitbucket.com/rest/",
			expectedBasePath: "https://company.bitbucket.com/rest",
		},
	} {
		result := NormalizeBitbucketBasePath(c.basePath)
		assert.Equal(t, c.expectedBasePath, result, c.testName)
	}
}
