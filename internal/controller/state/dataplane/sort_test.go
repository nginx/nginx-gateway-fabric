package dataplane

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/helpers"
)

func TestSortPathRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []PathRule
		expected []PathRule
	}{
		{
			name:     "empty slice",
			input:    []PathRule{},
			expected: []PathRule{},
		},
		{
			name:     "single element",
			input:    []PathRule{{Path: "/api", PathType: PathTypePrefix}},
			expected: []PathRule{{Path: "/api", PathType: PathTypePrefix}},
		},
		{
			name: "regex paths sorted by descending length",
			input: []PathRule{
				{Path: "/.*", PathType: PathTypeRegularExpression},
				{Path: "/api/.*", PathType: PathTypeRegularExpression},
				{Path: "/api/v1/users/[0-9]+", PathType: PathTypeRegularExpression},
			},
			expected: []PathRule{
				{Path: "/api/v1/users/[0-9]+", PathType: PathTypeRegularExpression},
				{Path: "/api/.*", PathType: PathTypeRegularExpression},
				{Path: "/.*", PathType: PathTypeRegularExpression},
			},
		},
		{
			name: "mixed path types sorted by descending length",
			input: []PathRule{
				{Path: "/", PathType: PathTypePrefix},
				{Path: "/.*", PathType: PathTypeRegularExpression},
				{Path: "/api", PathType: PathTypePrefix},
				{Path: "/api/v[0-9]+/.*", PathType: PathTypeRegularExpression},
				{Path: "/health", PathType: PathTypeExact},
			},
			expected: []PathRule{
				{Path: "/api/v[0-9]+/.*", PathType: PathTypeRegularExpression},
				{Path: "/health", PathType: PathTypeExact},
				{Path: "/api", PathType: PathTypePrefix},
				{Path: "/.*", PathType: PathTypeRegularExpression},
				{Path: "/", PathType: PathTypePrefix},
			},
		},
		{
			name: "same length paths use alphabetical tiebreak",
			input: []PathRule{
				{Path: "/bbb", PathType: PathTypePrefix},
				{Path: "/aaa", PathType: PathTypePrefix},
			},
			expected: []PathRule{
				{Path: "/aaa", PathType: PathTypePrefix},
				{Path: "/bbb", PathType: PathTypePrefix},
			},
		},
		{
			name: "same path uses type tiebreak: exact < prefix < regex",
			input: []PathRule{
				{Path: "/api", PathType: PathTypeRegularExpression},
				{Path: "/api", PathType: PathTypePrefix},
				{Path: "/api", PathType: PathTypeExact},
			},
			expected: []PathRule{
				{Path: "/api", PathType: PathTypeExact},
				{Path: "/api", PathType: PathTypePrefix},
				{Path: "/api", PathType: PathTypeRegularExpression},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			sortPathRules(test.input)
			g.Expect(test.input).To(Equal(test.expected))
		})
	}
}

func TestSort(t *testing.T) {
	t.Parallel()
	// timestamps
	earlier := metav1.Now()
	later := metav1.NewTime(earlier.Add(1 * time.Second))

	earlierTimestampMeta := &metav1.ObjectMeta{
		Name:              "hr1",
		Namespace:         "test",
		CreationTimestamp: earlier,
	}
	laterTimestampMeta := &metav1.ObjectMeta{
		Name:              "hr2",
		Namespace:         "test",
		CreationTimestamp: later,
	}
	laterTimestampButAlphabeticallyFirstMeta := &metav1.ObjectMeta{
		Name:              "hr3",
		Namespace:         "a-test",
		CreationTimestamp: later,
	}

	pathOnly := MatchRule{
		Match:  Match{},
		Source: earlierTimestampMeta,
	}
	twoHeadersEarlierTimestamp := MatchRule{
		Match: Match{
			Headers: []HTTPHeaderMatch{
				{
					Name:  "header1",
					Value: "value1",
				},
				{
					Name:  "header2",
					Value: "value2",
				},
			},
		},
		Source: earlierTimestampMeta,
	}
	twoHeadersOneParam := MatchRule{
		Match: Match{
			Headers: []HTTPHeaderMatch{
				{
					Name:  "header1",
					Value: "value1",
				},
				{
					Name:  "header2",
					Value: "value2",
				},
			},
			QueryParams: []HTTPQueryParamMatch{
				{
					Name:  "key1",
					Value: "value1",
				},
			},
		},
		Source: earlierTimestampMeta,
	}
	threeHeaders := MatchRule{
		Match: Match{
			Headers: []HTTPHeaderMatch{
				{
					Name:  "header1",
					Value: "value1",
				},
				{
					Name:  "header2",
					Value: "value2",
				},
				{
					Name:  "header3",
					Value: "value3",
				},
			},
		},
		Source: earlierTimestampMeta,
	}
	methodEarlierTimestamp := MatchRule{
		Match: Match{
			Method: helpers.GetPointer("POST"),
		},
		Source: earlierTimestampMeta,
	}
	methodLaterTimestamp := MatchRule{
		Match: Match{
			Method: helpers.GetPointer("POST"),
		},
		Source: earlierTimestampMeta,
	}
	twoHeadersLaterTimestamp := MatchRule{
		Match: Match{
			Headers: []HTTPHeaderMatch{
				{
					Name:  "header1",
					Value: "value1",
				},
				{
					Name:  "header2",
					Value: "value2",
				},
			},
		},
		Source: laterTimestampMeta,
	}
	twoHeadersLaterTimestampButAlphabeticallyBefore := MatchRule{
		Match: Match{
			Headers: []HTTPHeaderMatch{
				{
					Name:  "header1",
					Value: "value1",
				},
				{
					Name:  "header2",
					Value: "value2",
				},
			},
		},
		Source: laterTimestampButAlphabeticallyFirstMeta,
	}

	rules := []MatchRule{
		methodLaterTimestamp,
		pathOnly,
		twoHeadersEarlierTimestamp,
		twoHeadersOneParam,
		threeHeaders,
		methodEarlierTimestamp,
		twoHeadersLaterTimestamp,
		twoHeadersLaterTimestampButAlphabeticallyBefore,
	}

	sortedRules := []MatchRule{
		methodEarlierTimestamp,
		methodLaterTimestamp,
		threeHeaders,
		twoHeadersOneParam,
		twoHeadersEarlierTimestamp,
		twoHeadersLaterTimestampButAlphabeticallyBefore,
		twoHeadersLaterTimestamp,
		pathOnly,
	}

	sortMatchRules(rules)

	g := NewWithT(t)
	g.Expect(cmp.Diff(sortedRules, rules)).To(BeEmpty())
}
