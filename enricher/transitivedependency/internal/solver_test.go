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

package internal_test

import (
	"testing"

	"deps.dev/util/resolve"
	"github.com/google/go-cmp/cmp"
	"github.com/google/osv-scalibr/enricher/transitivedependency/internal"
)

func TestPackageUnwrapFn(t *testing.T) {
	want := directDependency(resolve.NPM, "bar", "1.0.0")

	solver, err := internal.NewSolver([]*fakePackage{{nil}, {want}}, fakeUnwrap)
	if err != nil {
		t.Fatalf("NewSolver failed: %v", err)
	}

	got, err := solver.Solve(t.Context(), requirement(resolve.NPM, "bar", "1.0.0"))
	if err != nil {
		t.Fatalf("Requirements failed: %v", err)
	}

	if diff := cmp.Diff([]*fakePackage{{want}}, got); diff != "" {
		t.Errorf("NewSolver did not fail on nil unwrap, but returned unexpected diff (-want +got):\n%s", diff)
	}
}

func TestRequirements(t *testing.T) {
	pkgFoo := &fakePackage{
		directDependency(resolve.NPM, "foo", "1.0.0", requirement(resolve.NPM, "bar", "1.0.0")),
	}
	pkgBar := &fakePackage{directDependency(resolve.NPM, "bar", "1.0.0")}
	pkgBaz := &fakePackage{directDependency(resolve.NPM, "baz", "1.0.0")}

	testCases := []struct {
		name     string
		packages []*fakePackage
		req      *fakePackage
		want     []resolve.RequirementVersion
		wantErr  bool
	}{
		{
			name:     "empty",
			packages: nil,
			req:      pkgFoo,
			wantErr:  true,
		},
		{
			name:     "no requirements",
			packages: []*fakePackage{pkgBaz},
			req:      pkgBaz,
			want:     nil,
		},
		{
			name:     "has requirements",
			packages: []*fakePackage{pkgFoo, pkgBar},
			req:      pkgFoo,
			want: []resolve.RequirementVersion{
				requirement(resolve.NPM, "bar", "1.0.0"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			solver, err := internal.NewSolver(tc.packages, fakeUnwrap)
			if err != nil {
				t.Fatalf("NewSolver failed: %v", err)
			}

			got, err := solver.Requirements(t.Context(), tc.req)
			if tc.wantErr && err == nil {
				t.Fatalf("Requirements succeeded, expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Requirements failed: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Requirements returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSolve(t *testing.T) {
	testCases := []struct {
		name     string
		packages []*fakePackage
		req      resolve.RequirementVersion
		want     []*fakePackage
	}{
		{
			name:     "empty",
			packages: nil,
			req:      requirement(resolve.NPM, "foo", "1.0.0"),
			want:     nil,
		},
		{
			name: "full match",
			packages: []*fakePackage{
				{directDependency(resolve.NPM, "foo", "1.0.0")},
				{directDependency(resolve.NPM, "foo", "2.1.3")},
				{directDependency(resolve.NPM, "bar", "5.0.0")},
			},
			req: requirement(resolve.NPM, "foo", ">=2.0.0"),
			want: []*fakePackage{{
				directDependency(resolve.NPM, "foo", "2.1.3"),
			}},
		},
		{
			name: "not found",
			packages: []*fakePackage{
				{directDependency(resolve.NPM, "foo", "1.0.0")},
				{directDependency(resolve.NPM, "foo", "2.1.3")},
				{directDependency(resolve.NPM, "bar", "5.0.0")},
			},
			req:  requirement(resolve.NPM, "baz", ""),
			want: nil,
		},
		{
			name: "no-version match",
			packages: []*fakePackage{
				{directDependency(resolve.NPM, "foo", "1.0.0")},
				{directDependency(resolve.NPM, "foo", "2.0.0")},
				{directDependency(resolve.NPM, "bar", "5.0.0")},
			},
			req: requirement(resolve.NPM, "foo", ""),
			// The solver was not given any restriction on the version, so it returned all known ones.
			want: []*fakePackage{
				{directDependency(resolve.NPM, "foo", "1.0.0")},
				{directDependency(resolve.NPM, "foo", "2.0.0")},
			},
		},
		{
			name: "unsatisfiable match",
			packages: []*fakePackage{
				{directDependency(resolve.NPM, "foo", "1.0.0")},
				{directDependency(resolve.NPM, "foo", "2.0.0")},
				{directDependency(resolve.NPM, "bar", "5.0.0")},
			},
			req: requirement(resolve.NPM, "foo", "4.0.0"),
			// The solver knew about this package, but the required constraint was unsatisfiable.
			want: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			solver, err := internal.NewSolver(tc.packages, fakeUnwrap)
			if err != nil {
				t.Fatalf("NewSolver failed: %v", err)
			}

			got, err := solver.Solve(t.Context(), tc.req)
			if err != nil {
				t.Fatalf("Solve failed: %v", err)
			}

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("Solve returned unexpected diff (-want +got):\n%s", diff)
			}
		})
	}
}

type fakePackage struct {
	Dependency *internal.DirectDependency
}

func fakeUnwrap(pkg *fakePackage) *internal.DirectDependency {
	return pkg.Dependency
}

func directDependency(system resolve.System, name, version string, requirements ...resolve.RequirementVersion) *internal.DirectDependency {
	return &internal.DirectDependency{
		Version: resolve.Version{
			VersionKey: resolve.VersionKey{
				PackageKey:  resolve.PackageKey{System: system, Name: name},
				VersionType: resolve.Concrete,
				Version:     version,
			},
		},
		Requirements: requirements,
	}
}

func requirement(system resolve.System, name, version string) resolve.RequirementVersion {
	return resolve.RequirementVersion{
		VersionKey: resolve.VersionKey{
			PackageKey:  resolve.PackageKey{System: system, Name: name},
			VersionType: resolve.Requirement,
			Version:     version,
		},
	}
}
