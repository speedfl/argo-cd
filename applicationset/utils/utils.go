package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"text/template"

	log "github.com/sirupsen/logrus"

	argoappsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappsetv1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

type Renderer interface {
	RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsetv1.ApplicationSetSyncPolicy, params map[string]interface{}) (*argoappsv1.Application, error)
}

type Render struct {
}

func (r *Render) RenderTemplateParams(tmpl *argoappsv1.Application, syncPolicy *argoappsetv1.ApplicationSetSyncPolicy, data map[string]interface{}) (*argoappsv1.Application, error) {
	if tmpl == nil {
		return nil, fmt.Errorf("application template is empty ")
	}

	if len(data) == 0 {
		return tmpl, nil
	}

	tmplBytes, err := json.Marshal(tmpl)
	if err != nil {
		return nil, err
	}

	finalTemplate, err := r.getTemplate(string(tmplBytes))

	if err != nil {
		return nil, err
	}

	template, err := template.New(tmpl.Name).Parse(finalTemplate)

	if err != nil {
		return nil, err
	}

	var replacedTmplBuffer bytes.Buffer

	if err := template.Execute(&replacedTmplBuffer, data); err != nil {
		return nil, err
	}

	var replacedTmpl argoappsv1.Application
	err = json.Unmarshal(replacedTmplBuffer.Bytes(), &replacedTmpl)
	if err != nil {
		return nil, err
	}

	// Add the 'resources-finalizer' finalizer if:
	// The template application doesn't have any finalizers, and:
	// a) there is no syncPolicy, or
	// b) there IS a syncPolicy, but preserveResourcesOnDeletion is set to false
	// See TestRenderTemplateParamsFinalizers in util_test.go for test-based definition of behaviour
	if (syncPolicy == nil || !syncPolicy.PreserveResourcesOnDeletion) &&
		(replacedTmpl.ObjectMeta.Finalizers == nil || len(replacedTmpl.ObjectMeta.Finalizers) == 0) {

		replacedTmpl.ObjectMeta.Finalizers = []string{"resources-finalizer.argocd.argoproj.io"}
	}

	return &replacedTmpl, nil
}

// Check if input is matching Go Template
func (r *Render) isMatchingGoTemplate(input string) bool {
	match := regexp.MustCompile(`{{\s*-?(?:and |call |html |index |slice |js |len |not |or |print |printf |println |urlquery |eq |ge |gt |le |lt |ne |block |break |continue |define |else |end |if |range |nil |template |with |".*?"|\$|.*?\(.*?\)|\.).*?-?}}`).Find([]byte(input))
	return match != nil
}

// For backward compatibility, ensure previous flat templating is still working by performing following changes:
// {{ path[n] }} => {{ .path.segments[n] }}
// {{ path }} => {{ .path.path }}
// {{ anything.with.a.dot }} => {{ .anything.with.a.dot }}
func (r *Render) replaceLegacyFlat(input string) string {

	var tmp string

	// Step 1: {{ path[n] }} to {{ .path.segments[n] }}
	rePathSegments := regexp.MustCompile(`{{\s*path\[(.*?)\]\s*}}`)
	tmp = rePathSegments.ReplaceAllString(input, "{{ .path.segments[${1}] }}")

	// Step 2: {{ path }} to {{ .path.path }}
	rePath := regexp.MustCompile(`{{\s*path\s*}}`)
	tmp = rePath.ReplaceAllString(tmp, "{{ .path.path }}")

	// Step 3: {{ anything.with.a.dot }} => {{ .anything.with.a.dot }}
	// golang does not negative support lookahead, so cannot use {{\s*(?!\.).*?\s*}}
	// will search all match of {{\s*(.*)\s*}} and if there is no "." do the replacement
	reAll := regexp.MustCompile(`{{\s*(.*?)?\s*}}`)
	matches := reAll.FindAllStringSubmatch(tmp, -1)

	if matches == nil {
		return tmp
	}

	for _, value := range matches {
		if !strings.HasPrefix(value[1], ".") {
			reReplace := regexp.MustCompile(fmt.Sprintf(`{{\s*(%s)\s*}}`, value[1]))
			tmp = reReplace.ReplaceAllString(tmp, "{{ .${1} }}")
		}
	}

	return tmp
}

// check if template is Go Template like
// if yes returns it as it
// if not apply logic to replace legacy params
func (r *Render) getTemplate(input string) (string, error) {

	if r.isMatchingGoTemplate(input) {
		return input, nil
	}

	return r.replaceLegacyFlat(input), nil
}

// Replace executes basic string substitution of a template with replacement values.
// 'allowUnresolved' indicates whether it is acceptable to have unresolved variables
// remaining in the substituted template.
func (r *Render) Replace(tmpl string, replaceMap any) (string, error) {

	finalTemplate, err := r.getTemplate(tmpl)

	if err != nil {
		return "", err
	}

	template, err := template.New("").Parse(finalTemplate)

	if err != nil {
		return "", err
	}

	var replacedTmplBuffer bytes.Buffer

	if err := template.Execute(&replacedTmplBuffer, replaceMap); err != nil {
		return "", nil
	}

	return replacedTmplBuffer.String(), nil
}

// Log a warning if there are unrecognized generators
func CheckInvalidGenerators(applicationSetInfo *argoappsetv1.ApplicationSet) {
	hasInvalidGenerators, invalidGenerators := invalidGenerators(applicationSetInfo)
	if len(invalidGenerators) > 0 {
		gnames := []string{}
		for n := range invalidGenerators {
			gnames = append(gnames, n)
		}
		sort.Strings(gnames)
		aname := applicationSetInfo.ObjectMeta.Name
		msg := "ApplicationSet %s contains unrecognized generators: %s"
		log.Warnf(msg, aname, strings.Join(gnames, ", "))
	} else if hasInvalidGenerators {
		name := applicationSetInfo.ObjectMeta.Name
		msg := "ApplicationSet %s contains unrecognized generators"
		log.Warnf(msg, name)
	}
}

// Return true if there are unknown generators specified in the application set.  If we can discover the names
// of these generators, return the names as the keys in a map
func invalidGenerators(applicationSetInfo *argoappsetv1.ApplicationSet) (bool, map[string]bool) {
	names := make(map[string]bool)
	hasInvalidGenerators := false
	for index, generator := range applicationSetInfo.Spec.Generators {
		v := reflect.Indirect(reflect.ValueOf(generator))
		found := false
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if !field.CanInterface() {
				continue
			}
			if !reflect.ValueOf(field.Interface()).IsNil() {
				found = true
				break
			}
		}
		if !found {
			hasInvalidGenerators = true
			addInvalidGeneratorNames(names, applicationSetInfo, index)
		}
	}
	return hasInvalidGenerators, names
}

func addInvalidGeneratorNames(names map[string]bool, applicationSetInfo *argoappsetv1.ApplicationSet, index int) {
	// The generator names are stored in the "kubectl.kubernetes.io/last-applied-configuration" annotation
	config := applicationSetInfo.ObjectMeta.Annotations["kubectl.kubernetes.io/last-applied-configuration"]
	var values map[string]interface{}
	err := json.Unmarshal([]byte(config), &values)
	if err != nil {
		log.Warnf("couldn't unmarshal kubectl.kubernetes.io/last-applied-configuration: %+v", config)
		return
	}

	spec, ok := values["spec"].(map[string]interface{})
	if !ok {
		log.Warn("coundn't get spec from kubectl.kubernetes.io/last-applied-configuration annotation")
		return
	}

	generators, ok := spec["generators"].([]interface{})
	if !ok {
		log.Warn("coundn't get generators from kubectl.kubernetes.io/last-applied-configuration annotation")
		return
	}

	if index >= len(generators) {
		log.Warnf("index %d out of range %d for generator in kubectl.kubernetes.io/last-applied-configuration", index, len(generators))
		return
	}

	generator, ok := generators[index].(map[string]interface{})
	if !ok {
		log.Warn("coundn't get generator from kubectl.kubernetes.io/last-applied-configuration annotation")
		return
	}

	for key := range generator {
		names[key] = true
		break
	}
}

func NormalizeBitbucketBasePath(basePath string) string {
	if strings.HasSuffix(basePath, "/rest/") {
		return strings.TrimSuffix(basePath, "/")
	}
	if !strings.HasSuffix(basePath, "/rest") {
		return basePath + "/rest"
	}
	return basePath
}
