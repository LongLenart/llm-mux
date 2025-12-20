package auth

import (
	"strings"
	"sync"
	"time"
)

// QuotaGroupResolver maps model IDs to their shared quota group.
// Models within the same quota group share rate limits - when one model
// hits quota, all models in the same group are blocked.
type QuotaGroupResolver func(provider, model string) string

var (
	quotaGroupMu        sync.RWMutex
	quotaGroupResolvers = make(map[string]QuotaGroupResolver)
	// quotaGroupProviders is a fast lookup set for providers that have quota grouping.
	// This avoids mutex lock for providers without grouping (common case).
	quotaGroupProviders = make(map[string]struct{})
)

// RegisterQuotaGroupResolver registers a custom quota group resolver for a provider.
// The resolver function receives provider and model, returns the quota group name.
// Return empty string for no grouping (each model has independent quota).
func RegisterQuotaGroupResolver(provider string, resolver QuotaGroupResolver) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" || resolver == nil {
		return
	}
	quotaGroupMu.Lock()
	quotaGroupResolvers[provider] = resolver
	quotaGroupProviders[provider] = struct{}{}
	quotaGroupMu.Unlock()
}

// HasQuotaGrouping returns true if the provider has quota grouping enabled.
// This is a fast check that avoids resolver lookup for providers without grouping.
func HasQuotaGrouping(provider string) bool {
	provider = strings.ToLower(provider)
	quotaGroupMu.RLock()
	_, ok := quotaGroupProviders[provider]
	quotaGroupMu.RUnlock()
	return ok
}

// ResolveQuotaGroup determines the quota group for a model under a provider.
// Models in the same quota group share rate limits.
// Returns empty string if no grouping applies.
func ResolveQuotaGroup(provider, model string) string {
	if provider == "" || model == "" {
		return ""
	}

	// Fast path: check if provider has grouping before acquiring lock
	providerLower := strings.ToLower(provider)
	quotaGroupMu.RLock()
	resolver, ok := quotaGroupResolvers[providerLower]
	quotaGroupMu.RUnlock()

	if !ok || resolver == nil {
		return ""
	}

	return resolver(providerLower, model)
}

// extractModelFamily extracts the model family prefix from a model ID.
// Examples:
//   - "claude-opus-4-5-thinking" -> "claude"
//   - "claude-sonnet-4" -> "claude"
//   - "gemini-2.5-pro" -> "gemini"
//   - "gpt-4o" -> "gpt"
func extractModelFamily(model string) string {
	if model == "" {
		return ""
	}

	// Fast path: find first delimiter without allocating
	modelLower := strings.ToLower(model)
	for i, r := range modelLower {
		if r == '-' || r == '_' || r == '.' {
			if i > 0 {
				return modelLower[:i]
			}
			return ""
		}
	}
	// No delimiter found, return entire model name as family
	return modelLower
}

// AntigravityQuotaGroupResolver groups models by their family prefix.
// This is used for providers like Antigravity where quota is shared
// across model families (all Claude models share quota, all Gemini models share quota, etc.)
func AntigravityQuotaGroupResolver(provider, model string) string {
	return extractModelFamily(model)
}

// quotaGroupIndex maintains a reverse index from quota group to blocked state.
// This enables O(1) lookup instead of O(N) iteration over ModelStates.
type quotaGroupIndex struct {
	// blockedGroups maps quota group name to its blocked state
	blockedGroups map[string]*quotaGroupState
}

type quotaGroupState struct {
	// NextRetryAfter is the earliest time any model in this group can retry
	NextRetryAfter time.Time
	// NextRecoverAt is when quota recovers
	NextRecoverAt time.Time
	// SourceModel is the model that originally triggered the quota block
	SourceModel string
}

// getOrCreateQuotaGroupIndex returns the quota group index from auth.Runtime,
// creating it if necessary.
func getOrCreateQuotaGroupIndex(auth *Auth) *quotaGroupIndex {
	if auth == nil {
		return nil
	}
	if auth.Runtime == nil {
		idx := &quotaGroupIndex{
			blockedGroups: make(map[string]*quotaGroupState),
		}
		auth.Runtime = idx
		return idx
	}
	if idx, ok := auth.Runtime.(*quotaGroupIndex); ok {
		return idx
	}
	// Runtime is used for something else, create wrapper
	idx := &quotaGroupIndex{
		blockedGroups: make(map[string]*quotaGroupState),
	}
	auth.Runtime = idx
	return idx
}

// getQuotaGroupIndex returns the quota group index from auth.Runtime if it exists.
func getQuotaGroupIndex(auth *Auth) *quotaGroupIndex {
	if auth == nil || auth.Runtime == nil {
		return nil
	}
	idx, _ := auth.Runtime.(*quotaGroupIndex)
	return idx
}

// setGroupBlocked marks a quota group as blocked.
func (idx *quotaGroupIndex) setGroupBlocked(group, sourceModel string, nextRetry, nextRecover time.Time) {
	if idx == nil || group == "" {
		return
	}
	if idx.blockedGroups == nil {
		idx.blockedGroups = make(map[string]*quotaGroupState)
	}
	idx.blockedGroups[group] = &quotaGroupState{
		NextRetryAfter: nextRetry,
		NextRecoverAt:  nextRecover,
		SourceModel:    sourceModel,
	}
}

// clearGroup removes a quota group from blocked state.
func (idx *quotaGroupIndex) clearGroup(group string) {
	if idx == nil || idx.blockedGroups == nil {
		return
	}
	delete(idx.blockedGroups, group)
}

// isGroupBlocked checks if a quota group is blocked.
// Returns (blocked, nextRetryAfter) - O(1) lookup.
func (idx *quotaGroupIndex) isGroupBlocked(group string, now time.Time) (bool, time.Time) {
	if idx == nil || idx.blockedGroups == nil || group == "" {
		return false, time.Time{}
	}
	state, ok := idx.blockedGroups[group]
	if !ok || state == nil {
		return false, time.Time{}
	}
	// Check if still blocked
	if state.NextRetryAfter.After(now) {
		next := state.NextRetryAfter
		if !state.NextRecoverAt.IsZero() && state.NextRecoverAt.After(now) && state.NextRecoverAt.After(next) {
			next = state.NextRecoverAt
		}
		return true, next
	}
	// Expired, clean up
	delete(idx.blockedGroups, group)
	return false, time.Time{}
}

// init registers default quota group resolvers for known providers
func init() {
	// Antigravity: Claude models share quota, Gemini models share quota
	RegisterQuotaGroupResolver("antigravity", AntigravityQuotaGroupResolver)
}
