// Copyright (C) 2017 The Android Open Source Project
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hidl

import (
	"sync"

	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"

	"android/soong/android"
	"android/soong/bazel"
)

var (
	currentTxtRule = pctx.StaticRule("currentTxtRule", blueprint.RuleParams{
		Command:     "cp -f ${in} ${output}",
		Description: "copy current.txt: ${in} => ${output}",
	}, "output")
)

type hidlPackageRootProperties struct {
	// Path to the package root from android build root. It is recommended not to set this and
	// use the current path. This will be deprecated in the future.
	Path *string

	// True to require a current.txt API file here.
	//
	// When false, it uses the file only when it exists.
	Use_current *bool

	// True to require all things referenced by this package root to be frozen.
	Require_frozen *bool
}

func init() {
	android.RegisterModuleType("hidl_package_root", HidlPackageRootFactory)
}

type hidlPackageRoot struct {
	android.ModuleBase
	android.BazelModuleBase

	properties hidlPackageRootProperties

	currentPath android.OptionalPath
	genOutputs  android.Paths
}

var _ android.SourceFileProducer = (*hidlPackageRoot)(nil)

func (r *hidlPackageRoot) getFullPackageRoot() string {
	return "-r" + r.Name() + ":" + *r.properties.Path
}

func (r *hidlPackageRoot) getCurrentPath() android.OptionalPath {
	return r.currentPath
}

func (r *hidlPackageRoot) requireFrozen() bool {
	return proptools.BoolDefault(r.properties.Require_frozen, false)
}

func (r *hidlPackageRoot) generateCurrentFile(ctx android.ModuleContext) {
	if !r.currentPath.Valid() {
		return
	}

	output := android.PathForModuleGen(ctx, r.Name()+".txt")
	r.genOutputs = append(r.genOutputs, output)

	ctx.ModuleBuild(pctx, android.ModuleBuildParams{
		Rule:   currentTxtRule,
		Input:  r.currentPath.Path(),
		Output: output,
		Args: map[string]string{
			"output": output.String(),
		},
	})
}

func (r *hidlPackageRoot) Srcs() android.Paths {
	return r.genOutputs
}

type hidlPackageRootAttributes struct {
	Path    *string
	Current bazel.LabelAttribute
}

func (r *hidlPackageRoot) ConvertWithBp2build(ctx android.Bp2buildMutatorContext) {
	attrs := &hidlPackageRootAttributes{
		Path: r.properties.Path,
	}
	currentPath := android.ExistentPathForSource(ctx, ctx.ModuleDir(), "current.txt")
	if currentPath.Valid() {
		attrs.Current = *bazel.MakeLabelAttribute("current.txt")
	}

	props := bazel.BazelTargetModuleProperties{
		Rule_class:        "hidl_package_root",
		Bzl_load_location: "//build/bazel/rules/hidl:hidl_package_root.bzl",
	}

	ctx.CreateBazelTargetModule(props, android.CommonAttributes{Name: ctx.ModuleName()}, attrs)

}

func (r *hidlPackageRoot) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	if r.properties.Path == nil {
		r.properties.Path = proptools.StringPtr(ctx.ModuleDir())
	}

	if proptools.BoolDefault(r.properties.Use_current, false) {
		if *r.properties.Path != ctx.ModuleDir() {
			ctx.PropertyErrorf("path", "Cannot use unrelated path with use_current. "+
				"Presumably this hidl_package_root should be at %s. Otherwise, current.txt "+
				"could be located at %s, but use_current must be removed. path is by default "+
				"the path of hidl_package_root.", *r.properties.Path, ctx.ModuleDir())
			return
		}

		r.currentPath = android.OptionalPathForPath(android.PathForModuleSrc(ctx, "current.txt"))
	} else {
		r.currentPath = android.ExistentPathForSource(ctx, ctx.ModuleDir(), "current.txt")
	}

	r.generateCurrentFile(ctx)
}

func (r *hidlPackageRoot) DepsMutator(ctx android.BottomUpMutatorContext) {
}

var packageRootsMutex sync.Mutex
var packageRoots []*hidlPackageRoot

func HidlPackageRootFactory() android.Module {
	r := &hidlPackageRoot{}
	r.AddProperties(&r.properties)
	android.InitAndroidModule(r)
	android.InitBazelModule(r)

	packageRootsMutex.Lock()
	packageRoots = append(packageRoots, r)
	packageRootsMutex.Unlock()

	return r
}

func lookupPackageRoot(name string) *hidlPackageRoot {
	for _, i := range packageRoots {
		if i.ModuleBase.Name() == name {
			return i
		}
	}
	return nil
}
