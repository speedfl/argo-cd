package generators

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestMatrixGenerate(t *testing.T) {

	gitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:     "RepoURL",
		Revision:    "Revision",
		Directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
	}

	listGenerator := &argoprojiov1alpha1.ListGenerator{
		Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "Cluster","url": "Url"}`)}},
	}

	testCases := []struct {
		name           string
		baseGenerators []argoprojiov1alpha1.ApplicationSetNestedGenerator
		expectedErr    error
		expected       []map[string]interface{}
	}{
		{
			name: "happy flow - generate params",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					List: listGenerator,
				},
			},
			expected: []map[string]interface{}{
				{
					"path": map[string]string{
						"path":               "app1",
						"basename":           "app1",
						"basenameNormalized": "app1",
					},
					"cluster": "Cluster",
					"url":     "Url",
				},
				{
					"path": map[string]string{
						"path":               "app2",
						"basename":           "app2",
						"basenameNormalized": "app2",
					},
					"cluster": "Cluster",
					"url":     "Url",
				},
			},
		},
		{
			name: "happy flow - generate params from two lists",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					List: &argoprojiov1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{
							{Raw: []byte(`{"a": "1"}`)},
							{Raw: []byte(`{"a": "2"}`)},
						},
					},
				},
				{
					List: &argoprojiov1alpha1.ListGenerator{
						Elements: []apiextensionsv1.JSON{
							{Raw: []byte(`{"b": "1"}`)},
							{Raw: []byte(`{"b": "2"}`)},
						},
					},
				},
			},
			expected: []map[string]interface{}{
				{"a": "1", "b": "1"},
				{"a": "1", "b": "2"},
				{"a": "2", "b": "1"},
				{"a": "2", "b": "2"},
			},
		},
		{
			name: "returns error if there is less than two base generators",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
			},
			expectedErr: ErrLessThanTwoGenerators,
		},
		{
			name: "returns error if there is more than two base generators",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					List: listGenerator,
				},
				{
					List: listGenerator,
				},
				{
					List: listGenerator,
				},
			},
			expectedErr: ErrMoreThanTwoGenerators,
		},
		{
			name: "returns error if there is more than one inner generator in the first base generator",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git:  gitGenerator,
					List: listGenerator,
				},
				{
					Git: gitGenerator,
				},
			},
			expectedErr: ErrMoreThenOneInnerGenerators,
		},
		{
			name: "returns error if there is more than one inner generator in the second base generator",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					List: listGenerator,
				},
				{
					Git:  gitGenerator,
					List: listGenerator,
				},
			},
			expectedErr: ErrMoreThenOneInnerGenerators,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			genMock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{}

			for _, g := range testCaseCopy.baseGenerators {

				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:  g.Git,
					List: g.List,
				}
				genMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), appSet).Return([]map[string]interface{}{
					{
						"path": map[string]string{
							"path":               "app1",
							"basename":           "app1",
							"basenameNormalized": "app1",
						},
					},
					{
						"path": map[string]string{
							"path":               "app2",
							"basename":           "app2",
							"basenameNormalized": "app2",
						},
					},
				}, nil)

				genMock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}

			var matrixGenerator = NewMatrixGenerator(
				map[string]Generator{
					"Git":  genMock,
					"List": &ListGenerator{},
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet)

			if testCaseCopy.expectedErr != nil {
				assert.EqualError(t, err, testCaseCopy.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

		})

	}
}

func TestMatrixGetRequeueAfter(t *testing.T) {

	gitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:     "RepoURL",
		Revision:    "Revision",
		Directories: []argoprojiov1alpha1.GitDirectoryGeneratorItem{{Path: "*"}},
	}

	listGenerator := &argoprojiov1alpha1.ListGenerator{
		Elements: []apiextensionsv1.JSON{{Raw: []byte(`{"cluster": "Cluster","url": "Url"}`)}},
	}

	testCases := []struct {
		name               string
		baseGenerators     []argoprojiov1alpha1.ApplicationSetNestedGenerator
		gitGetRequeueAfter time.Duration
		expected           time.Duration
	}{
		{
			name: "return NoRequeueAfter if all the inner baseGenerators returns it",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					List: listGenerator,
				},
			},
			gitGetRequeueAfter: NoRequeueAfter,
			expected:           NoRequeueAfter,
		},
		{
			name: "returns the minimal time",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: gitGenerator,
				},
				{
					List: listGenerator,
				},
			},
			gitGetRequeueAfter: time.Duration(1),
			expected:           time.Duration(1),
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			mock := &generatorMock{}

			for _, g := range testCaseCopy.baseGenerators {
				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:  g.Git,
					List: g.List,
				}
				mock.On("GetRequeueAfter", &gitGeneratorSpec).Return(testCaseCopy.gitGetRequeueAfter, nil)
			}

			var matrixGenerator = NewMatrixGenerator(
				map[string]Generator{
					"Git":  mock,
					"List": &ListGenerator{},
				},
			)

			got := matrixGenerator.GetRequeueAfter(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			})

			assert.Equal(t, testCaseCopy.expected, got)

		})

	}
}

func TestInterpolatedMatrixGenerate(t *testing.T) {
	interpolatedGitGenerator := &argoprojiov1alpha1.GitGenerator{
		RepoURL:  "RepoURL",
		Revision: "Revision",
		Files: []argoprojiov1alpha1.GitFileGeneratorItem{
			{Path: "examples/git-generator-files-discovery/cluster-config/dev/config.json"},
			{Path: "examples/git-generator-files-discovery/cluster-config/prod/config.json"},
		},
	}

	interpolatedClusterGenerator := &argoprojiov1alpha1.ClusterGenerator{
		Selector: metav1.LabelSelector{
			MatchLabels:      map[string]string{"environment": "{{.path.basename}}"},
			MatchExpressions: nil,
		},
	}
	testCases := []struct {
		name           string
		baseGenerators []argoprojiov1alpha1.ApplicationSetNestedGenerator
		expectedErr    error
		expected       []map[string]interface{}
		clientError    bool
	}{
		{
			name: "happy flow - generate interpolated params",
			baseGenerators: []argoprojiov1alpha1.ApplicationSetNestedGenerator{
				{
					Git: interpolatedGitGenerator,
				},
				{
					Clusters: interpolatedClusterGenerator,
				},
			},
			expected: []map[string]interface{}{
				{
					"path": map[string]string{
						"path":               "examples/git-generator-files-discovery/cluster-config/dev/config.json",
						"basename":           "dev",
						"basenameNormalized": "dev",
					},
					"name":           "dev-01",
					"nameNormalized": "dev-01",
					"server":         "https://dev-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"environment":                    "dev",
							"argocd.argoproj.io/secret-type": "cluster",
						},
					},
				},
				{
					"path": map[string]string{
						"path":               "examples/git-generator-files-discovery/cluster-config/prod/config.json",
						"basename":           "prod",
						"basenameNormalized": "prod",
					},
					"name":           "prod-01",
					"nameNormalized": "prod-01",
					"server":         "https://prod-01.example.com",
					"metadata": map[string]interface{}{
						"labels": map[string]string{
							"environment":                    "prod",
							"argocd.argoproj.io/secret-type": "cluster",
						},
					},
				},
			},
			clientError: false,
		},
	}
	clusters := []client.Object{
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "dev",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("dev-01"),
				"server": []byte("https://dev-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
		&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prod-01",
				Namespace: "namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "cluster",
					"environment":                    "prod",
				},
			},
			Data: map[string][]byte{
				"config": []byte("{}"),
				"name":   []byte("prod-01"),
				"server": []byte("https://prod-01.example.com"),
			},
			Type: corev1.SecretType("Opaque"),
		},
	}
	// convert []client.Object to []runtime.Object, for use by kubefake package
	runtimeClusters := []runtime.Object{}
	for _, clientCluster := range clusters {
		runtimeClusters = append(runtimeClusters, clientCluster)
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase // Since tests may run in parallel

		t.Run(testCaseCopy.name, func(t *testing.T) {
			genMock := &generatorMock{}
			appSet := &argoprojiov1alpha1.ApplicationSet{}

			appClientset := kubefake.NewSimpleClientset(runtimeClusters...)
			fakeClient := fake.NewClientBuilder().WithObjects(clusters...).Build()
			cl := &possiblyErroringFakeCtrlRuntimeClient{
				fakeClient,
				testCase.clientError,
			}
			var clusterGenerator = NewClusterGenerator(cl, context.Background(), appClientset, "namespace")

			for _, g := range testCaseCopy.baseGenerators {

				gitGeneratorSpec := argoprojiov1alpha1.ApplicationSetGenerator{
					Git:      g.Git,
					Clusters: g.Clusters,
				}
				genMock.On("GenerateParams", mock.AnythingOfType("*v1alpha1.ApplicationSetGenerator"), appSet).Return([]map[string]interface{}{

					{
						"path": map[string]string{
							"path":               "examples/git-generator-files-discovery/cluster-config/dev/config.json",
							"basename":           "dev",
							"basenameNormalized": "dev",
						},
					},
					{
						"path": map[string]string{
							"path":               "examples/git-generator-files-discovery/cluster-config/prod/config.json",
							"basename":           "prod",
							"basenameNormalized": "prod",
						},
					},
				}, nil)
				genMock.On("GetTemplate", &gitGeneratorSpec).
					Return(&argoprojiov1alpha1.ApplicationSetTemplate{})
			}
			var matrixGenerator = NewMatrixGenerator(
				map[string]Generator{
					"Git":      genMock,
					"Clusters": clusterGenerator,
				},
			)

			got, err := matrixGenerator.GenerateParams(&argoprojiov1alpha1.ApplicationSetGenerator{
				Matrix: &argoprojiov1alpha1.MatrixGenerator{
					Generators: testCaseCopy.baseGenerators,
					Template:   argoprojiov1alpha1.ApplicationSetTemplate{},
				},
			}, appSet)

			if testCaseCopy.expectedErr != nil {
				assert.EqualError(t, err, testCaseCopy.expectedErr.Error())
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}

		})

	}
}

type generatorMock struct {
	mock.Mock
}

func (g *generatorMock) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	args := g.Called(appSetGenerator)

	return args.Get(0).(*argoprojiov1alpha1.ApplicationSetTemplate)
}

func (g *generatorMock) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet) ([]map[string]interface{}, error) {
	args := g.Called(appSetGenerator, appSet)

	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (g *generatorMock) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	args := g.Called(appSetGenerator)

	return args.Get(0).(time.Duration)

}
