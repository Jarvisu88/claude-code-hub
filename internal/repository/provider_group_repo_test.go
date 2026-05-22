package repository

import (
	"testing"

	"github.com/quagmt/udecimal"
)

func TestParseProviderGroupsDeduplicatesAndNormalizes(t *testing.T) {
	got := parseProviderGroups(" premium, enterprise\npremium\rdefault\t, enterprise ")
	want := []string{"premium", "enterprise", "default"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestProviderGroupRepositoryCacheRoundTrip(t *testing.T) {
	repo := NewProviderGroupRepository(nil).(*providerGroupRepository)
	value := udecimal.MustParse("1.2500")
	repo.setCachedMultiplier("premium", value)
	got, ok := repo.getCachedMultiplier("premium")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if !got.Equal(value) {
		t.Fatalf("expected %s, got %s", value.String(), got.String())
	}
	repo.InvalidateCache()
	if _, ok := repo.getCachedMultiplier("premium"); ok {
		t.Fatal("expected cache to be invalidated")
	}
}
