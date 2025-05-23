package dataplane

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nginx/nginx-gateway-fabric/internal/framework/helpers"
)

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
