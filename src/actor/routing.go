package actor

import (
	"reflect"
	"sync"
)

const (
	defaultSchemaVersion = "v1"
	defaultHandlerKey    = "default"
)

type schemaVersionProvider interface {
	SchemaVersion() string
}

type routeMatch string

const (
	routeNoRules         routeMatch = "no_rules"
	routeExact           routeMatch = "exact"
	routeFallback        routeMatch = "fallback"
	routeUnsupportedType routeMatch = "unsupported_type"
	routeVersionMismatch routeMatch = "version_mismatch"
)

type routeResolution struct {
	match      routeMatch
	handlerKey string
}

type typeRoutingRule struct {
	typeName      string
	schemaVersion string
	handlerKey    string
	isFallback    bool
}

type routeKey struct {
	typeName      string
	schemaVersion string
}

type actorRoutes struct {
	exact    map[routeKey]typeRoutingRule
	versions map[string]map[string]struct{}
	fallback *typeRoutingRule
}

type typeRoutingRegistry struct {
	mu      sync.RWMutex
	byActor map[string]*actorRoutes
}

func newTypeRoutingRegistry() *typeRoutingRegistry {
	return &typeRoutingRegistry{byActor: map[string]*actorRoutes{}}
}

func (r *typeRoutingRegistry) ensure(actorID string) *actorRoutes {
	ar, ok := r.byActor[actorID]
	if !ok {
		ar = &actorRoutes{
			exact:    map[routeKey]typeRoutingRule{},
			versions: map[string]map[string]struct{}{},
		}
		r.byActor[actorID] = ar
	}
	return ar
}

func (r *typeRoutingRegistry) registerExact(actorID, typeName, schemaVersion, handlerKey string) {
	if typeName == "" {
		return
	}
	if schemaVersion == "" {
		schemaVersion = defaultSchemaVersion
	}
	if handlerKey == "" {
		handlerKey = defaultHandlerKey
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ar := r.ensure(actorID)
	key := routeKey{typeName: typeName, schemaVersion: schemaVersion}
	ar.exact[key] = typeRoutingRule{
		typeName:      typeName,
		schemaVersion: schemaVersion,
		handlerKey:    handlerKey,
		isFallback:    false,
	}
	if _, ok := ar.versions[typeName]; !ok {
		ar.versions[typeName] = map[string]struct{}{}
	}
	ar.versions[typeName][schemaVersion] = struct{}{}
}

func (r *typeRoutingRegistry) registerFallback(actorID, handlerKey string) {
	if handlerKey == "" {
		handlerKey = defaultHandlerKey
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	ar := r.ensure(actorID)
	ar.fallback = &typeRoutingRule{
		handlerKey: handlerKey,
		isFallback: true,
	}
}

func (r *typeRoutingRegistry) resolve(actorID, typeName, schemaVersion string) routeResolution {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ar, ok := r.byActor[actorID]
	if !ok || (len(ar.exact) == 0 && ar.fallback == nil) {
		return routeResolution{match: routeNoRules}
	}

	if schemaVersion == "" {
		schemaVersion = defaultSchemaVersion
	}
	key := routeKey{typeName: typeName, schemaVersion: schemaVersion}
	if rule, ok := ar.exact[key]; ok {
		return routeResolution{match: routeExact, handlerKey: rule.handlerKey}
	}
	if versions, ok := ar.versions[typeName]; ok && len(versions) > 0 {
		return routeResolution{match: routeVersionMismatch}
	}
	if ar.fallback != nil {
		return routeResolution{match: routeFallback, handlerKey: ar.fallback.handlerKey}
	}
	return routeResolution{match: routeUnsupportedType}
}

func messageTypeName(payload any) string {
	if payload == nil {
		return ""
	}
	return reflect.TypeOf(payload).String()
}

func messageSchemaVersion(payload any) string {
	if payload == nil {
		return defaultSchemaVersion
	}
	v, ok := payload.(schemaVersionProvider)
	if !ok {
		return defaultSchemaVersion
	}
	version := v.SchemaVersion()
	if version == "" {
		return defaultSchemaVersion
	}
	return version
}
