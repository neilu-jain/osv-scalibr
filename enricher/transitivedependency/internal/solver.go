// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"context"
	"errors"
	"fmt"

	"deps.dev/util/resolve"
)

// PackageUnwrapFn is a function that extracts a DirectDependency from a generic input format.
type PackageUnwrapFn[T comparable] func(T) *DirectDependency

// DirectDependency holds metadata about direct dependencies of a software package.
type DirectDependency struct {
	// Version is the version of the software package declaring a set of direct dependencies.
	Version resolve.Version
	// Requirements is the set of requested direct dependencies, with version constraints.
	Requirements []resolve.RequirementVersion
}

// Solver is a database of packages that can be used to resolve dependency constraints.
type Solver[T comparable] struct {
	client *resolve.LocalClient
	// packages maps version keys, as resolved by the client, to the input packages they were
	// associated with. The mapping is user-determined, based on the PackageUnwrapFn we received.
	packages map[resolve.VersionKey][]T
	// versions is the reverse mapping of packages.
	versions map[T]resolve.VersionKey
}

// NewSolver creates a new SolverDatabase from a list of (generic) packages.
func NewSolver[T comparable](packages []T, unwrap PackageUnwrapFn[T]) (*Solver[T], error) {
	solver := &Solver[T]{
		client:   resolve.NewLocalClient(),
		packages: make(map[resolve.VersionKey][]T, len(packages)),
		versions: make(map[T]resolve.VersionKey, len(packages)),
	}
	for _, p := range packages {
		meta := unwrap(p)
		if meta == nil {
			continue
		}
		key := meta.Version.VersionKey
		solver.client.AddVersion(meta.Version, meta.Requirements)
		solver.packages[key] = append(solver.packages[key], p)
		solver.versions[p] = key
	}
	return solver, nil
}

// Requirement represent a direct dependency, with an ecosystem-specific constraint on the version.
type Requirement = resolve.RequirementVersion

// Requirements returns the direct dependency constraints declared by the given package.
//
// If the package is not known, the solver will return [resolve.ErrNotFound].
func (s *Solver[T]) Requirements(ctx context.Context, pkg T) ([]Requirement, error) {
	key, ok := s.versions[pkg]
	if !ok {
		return nil, fmt.Errorf("package %v: %w", pkg, resolve.ErrNotFound)
	}
	return s.client.Requirements(ctx, key)
}

// Solve returns a list of (generic) packages that match the given dependency constraint.
//
// If no match was found, the solver will conservatively return all known versions.
//
// If the output is empty, then the dependency is not known to the solver. Generally, this means
// that it was not present in the artifact under analysis, or that SCALIBR was unable to extract
// any metadata for it. This is not necessarily an issue, e.g. a dependency could be optional,
// test-only, or have been eliminated based on effective usage. Each ecosystem has different ways
// to declare these special dependencies. The solver will not attempt to account for this.
func (s *Solver[T]) Solve(ctx context.Context, req Requirement) ([]T, error) {
	var versions []resolve.Version
	var err error
	if req.VersionKey.Version == "" {
		versions, err = s.client.Versions(ctx, req.PackageKey)
	} else {
		versions, err = s.client.MatchingVersions(ctx, req.VersionKey)
	}

	if errors.Is(err, resolve.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to solve %s: %w", req, err)
	}

	var packages []T
	for _, v := range versions {
		packages = append(packages, s.packages[v.VersionKey]...)
	}
	return packages, nil
}
