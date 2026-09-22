// Package godiscovery decodes the closed Go 1.27 package-discovery stream.
// It never executes Go or reads repository files; callers supply independently
// verified path, mode, and content commitments for every consumed input.
package godiscovery

import "fmt"

const (
	MaxDiscoveryBytes = 16 << 20
	MaxPackages       = 4_096
	MaxStringBytes    = 1 << 20
	MaxJSONDepth      = 16
	MaxArgvBytes      = 4 << 20
)

const ClosedFields = "Dir,ImportPath,Name,ForTest,Match,DepOnly,Module,GoFiles,CgoFiles,CFiles,CXXFiles,MFiles,HFiles,FFiles,SFiles,SwigFiles,SwigCXXFiles,SysoFiles,EmbedFiles,TestGoFiles,XTestGoFiles,TestEmbedFiles,XTestEmbedFiles,Imports,TestImports,XTestImports,Error,DepsErrors"

type Code string

const (
	CodeDecode Code = "DISCOVERY_DECODE"
	CodeInput  Code = "DISCOVERY_INPUT"
	CodeLimit  Code = "LIMIT_EXCEEDED"
)

type Error struct {
	Code Code
}

func (e *Error) Error() string {
	return fmt.Sprintf("go-live-discovery:%s", e.Code)
}

type PackageKind string

const (
	Dependency  PackageKind = "DEPENDENCY"
	Requested   PackageKind = "REQUESTED"
	TestMain    PackageKind = "TEST_MAIN"
	TestVariant PackageKind = "TEST_VARIANT"
)

type ModuleMode string

const (
	ModuleModeModule    ModuleMode = "MODULE"
	ModuleModeWorkspace ModuleMode = "WORKSPACE"
	ModuleModeVendor    ModuleMode = "VENDOR"
)

type InputRole string

const (
	RoleSource            InputRole = "SOURCE"
	RoleGeneratedTestMain InputRole = "GENERATED_TEST_MAIN"
)

type PathNamespace string

const (
	NamespaceSource     PathNamespace = "SOURCE"
	NamespaceDependency PathNamespace = "DEPENDENCY"
	NamespaceToolchain  PathNamespace = "TOOLCHAIN"
)

type FileField string

const (
	FieldGoFiles         FileField = "GoFiles"
	FieldCgoFiles        FileField = "CgoFiles"
	FieldCFiles          FileField = "CFiles"
	FieldCXXFiles        FileField = "CXXFiles"
	FieldMFiles          FileField = "MFiles"
	FieldHFiles          FileField = "HFiles"
	FieldFFiles          FileField = "FFiles"
	FieldSFiles          FileField = "SFiles"
	FieldSwigFiles       FileField = "SwigFiles"
	FieldSwigCXXFiles    FileField = "SwigCXXFiles"
	FieldSysoFiles       FileField = "SysoFiles"
	FieldEmbedFiles      FileField = "EmbedFiles"
	FieldTestGoFiles     FileField = "TestGoFiles"
	FieldXTestGoFiles    FileField = "XTestGoFiles"
	FieldTestEmbedFiles  FileField = "TestEmbedFiles"
	FieldXTestEmbedFiles FileField = "XTestEmbedFiles"
)

type FileCommitment struct {
	Field       FileField
	ListedName  string
	Role        InputRole
	Mode        string
	Namespace   PathNamespace
	LogicalPath string
	PathSHA256  string
	RawSHA256   string
}

type PackageMaterial struct {
	ImportPath     string
	RawDir         string
	DirNamespace   PathNamespace
	DirLogicalPath string
	DirPathSHA256  string
	Files          []FileCommitment
}

type PathCommitment struct {
	RawPath     string
	Namespace   PathNamespace
	LogicalPath string
	PathSHA256  string
}

type Commitments struct {
	DependencyMaterializationSHA256 string
	ModuleMode                      ModuleMode
	ModulePaths                     []PathCommitment
	Packages                        []PackageMaterial
	SourceWSI                       string
}

type Package struct {
	DepOnly          bool
	DirPathSHA256    string
	ForTest          *string
	ID               string
	ImportPath       string
	InputFiles       []InputFile
	InputFilesSHA256 string
	Imports          []string
	Kind             PackageKind
	Match            []string
	Module           *Module
	ModuleSHA256     *string
	Name             string
	TestImports      []string
	XTestImports     []string
}

type InputFile struct {
	Mode       string
	PathSHA256 string
	RawSHA256  string
}

type Module struct {
	DirPathSHA256   *string
	GoModPathSHA256 *string
	GoModSum        *string
	GoVersion       *string
	Indirect        bool
	Main            bool
	Path            string
	Replace         *Module
	Sum             *string
	TimeRaw         *string
	Version         *string
}

type Manifest struct {
	DependencyMaterializationSHA256 string
	InputsSHA256                    string
	ModuleMode                      ModuleMode
	Packages                        []Package
	RawStdoutBytes                  uint64
	RawStdoutSHA256                 string
	RequestedRunnerPackages         []string
	SourceWSI                       string
	seal                            string
}

type DocumentInput struct {
	EnvironmentSHA256 string
	PackagePatterns   []string
	RawStderrSHA256   string
	ToolchainID       string
}

type Document struct {
	Argv                            []string
	DependencyMaterializationSHA256 string
	EnvironmentSHA256               string
	ExitCode                        string
	ID                              string
	InputsSHA256                    string
	ModuleMode                      ModuleMode
	Packages                        []Package
	Profile                         string
	RawStderrSHA256                 string
	RawStdoutSHA256                 string
	RequestedRunnerPackages         []string
	SourceWSI                       string
	ToolchainID                     string
}
