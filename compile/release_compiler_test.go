package compile_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	bmrel "github.com/cloudfoundry/bosh-micro-cli/release"

	fakeboshcomp "github.com/cloudfoundry/bosh-micro-cli/compile/fakes"
	fakebmlog "github.com/cloudfoundry/bosh-micro-cli/logging/fakes"
	fakebmreal "github.com/cloudfoundry/bosh-micro-cli/release/fakes"

	. "github.com/cloudfoundry/bosh-micro-cli/compile"
)

var _ = Describe("ReleaseCompiler", func() {
	var (
		release         bmrel.Release
		releaseCompiler ReleaseCompiler
		da              *fakebmreal.FakeDependencyAnalysis
		packageCompiler *fakeboshcomp.FakePackageCompiler
		eventLogger     *fakebmlog.FakeEventLogger
	)

	BeforeEach(func() {
		da = fakebmreal.NewFakeDependencyAnalysis()
		packageCompiler = fakeboshcomp.NewFakePackageCompiler()
		eventLogger = fakebmlog.NewFakeEventLogger()
		releaseCompiler = NewReleaseCompiler(da, packageCompiler, eventLogger)
		release = bmrel.Release{}
	})

	Context("Compile", func() {
		Context("when the release", func() {
			var expectedPackages []*bmrel.Package
			var package1, package2 bmrel.Package

			BeforeEach(func() {
				package1 = bmrel.Package{Name: "fake-package-1"}
				package2 = bmrel.Package{Name: "fake-package-2"}

				expectedPackages = []*bmrel.Package{&package1, &package2}

				da.DeterminePackageCompilationOrderResult = []*bmrel.Package{
					&package1,
					&package2,
				}
			})

			It("determines the order to compile packages", func() {
				err := releaseCompiler.Compile(release)
				Expect(err).NotTo(HaveOccurred())
				Expect(da.DeterminePackageCompilationOrderRelease).To(Equal(release))
			})

			It("compiles each package", func() {
				err := releaseCompiler.Compile(release)
				Expect(err).NotTo(HaveOccurred())
				Expect(packageCompiler.CompilePackages).To(Equal(expectedPackages))
			})

			It("compiles each package and returns error for first package", func() {
				packageCompiler.CompileError = errors.New("Compilation failed")
				err := releaseCompiler.Compile(release)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Package `fake-package-1' compilation failed"))
			})

			It("logs each compilation event within a shared group", func() {
				err := releaseCompiler.Compile(release)
				Expect(err).ToNot(HaveOccurred())
				Expect(eventLogger.StartedGroup).To(Equal("compiling packages"))
				Expect(eventLogger.LoggedEvents).To(ContainElement(ContainSubstring(package1.Name)))
				Expect(eventLogger.LoggedEvents).To(ContainElement(ContainSubstring(package2.Name)))
				Expect(eventLogger.FinishGroupCalled).To(BeTrue())
			})

			It("stops compiling after the first failures", func() {
				packageCompiler.CompileError = errors.New("Compilation failed")
				err := releaseCompiler.Compile(release)
				Expect(err).To(HaveOccurred())
				Expect(len(packageCompiler.CompilePackages)).To(Equal(1))
			})
		})
	})
})