// Package parse finds interfaces in go files and parses them into a representation that is more convenient for code generation.
package parse

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/mod/modfile"
)

// Recursively searches the given path for interfaces.
func ParseDir(path string, module Module) ([]Interface, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	var result []Interface

	packages := findPackages(path, module)
	for _, pkg := range packages {
		i, err := findInterfacesInPackage(pkg)
		if err != nil {
			return nil, err
		}
		result = append(result, i...)
	}
	return result, nil
}

type Package struct {
	//path to the package in the filesystem
	FilePath string
	//full package path, i.e. the full name of the package e.g. "github.com/xyz/abc"
	PackagePath string
}

// Returns a list of packages contained in the given root directory.
func findPackages(root string, module Module) []Package {
	var result []Package
	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			packagePath, err := module.PackagePathFromFilePath(path)
			if err != nil {
				log.Println("could not build package path for file path:", path)
				return nil
			}
			result = append(result, Package{
				PackagePath: packagePath,
				FilePath:    path,
			})
		}
		return nil
	})
	return result
}

func findInterfacesInPackage(pkg Package) ([]Interface, error) {
	var result []Interface

	//parser.ParseDir does not work recursively, i.e. it will only consider files in the given directory and not any subdirectories.
	packageMap, err := parser.ParseDir(
		token.NewFileSet(),
		pkg.FilePath,
		func(fileInfo fs.FileInfo) bool {
			//exclude test files
			if strings.HasSuffix(fileInfo.Name(), "_test.go") {
				return false
			}
			return true
		},
		parser.AllErrors|parser.ParseComments,
	)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("could not parse directory %v, got error: %v", pkg.FilePath, err))
	}

	//there should at most be one package here, at least if the code that this is run on compiles
	//since a single directory cannot contain files for more than one package
	for _, p := range packageMap {
		for filename, f := range p.Files {
			//name of package directory should match package path, i.e. files for a package "example.com/xyz/abc" should be in a folder "abc" and each file should contain the line "package abc".
			base := path.Base(pkg.PackagePath)
			pname := f.Name.Name
			if base != pname {
				//log.Println("package direcotry name", base, "doesn't match package name declared in files", pname)
			} else {
				i, err := findInterfacesInFile(f, pkg.PackagePath)
				if err != nil {
					return nil, err
				}
				//set filename
				for j := 0; j < len(i); j++ {
					i[j].File = filename
				}
				result = append(result, i...)
			}
		}
	}

	return result, nil
}

// Module provides functions to convert relative package names and file paths
// to absolute ones, based on a module.
type Module struct {
	// root path of the module
	Path string
	// name of the module
	Name string
}

// Will find the module the given directory belongs to by searching for go.mod file
// in the directory and its parents.
func NewModuleFromDir(dir string) (Module, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Module{}, err
	}

	curr := dir
	for filepath.Base(curr) != curr {
		content, err := os.ReadFile(filepath.Join(curr, "go.mod"))
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				curr = filepath.Dir(curr)
			} else {
				return Module{}, errors.New(fmt.Sprintf("error while trying to locate module: %v", err))
			}
		} else {
			moduleName := modfile.ModulePath(content)
			if moduleName == "" {
				return Module{}, errors.New(fmt.Sprintf("error while trying to parse mod file: %v", err))
			}
			return Module{
				Path: curr,
				Name: moduleName,
			}, nil
		}
	}

	return Module{}, errors.New("no module found")
}

func (m Module) PackagePathWithoutModule(p string) string {
	result := strings.TrimPrefix(p, m.Name)
	return strings.TrimPrefix(result, "/")
}

// Returns the full package path for the given relative package, i.e. the module name is added as a prefix.
func (m Module) FullPackagePath(p string) string {
	return path.Join(m.Name, p)
}

// Absolute file path for the file in the given package.
func (m Module) FileName(packageName, fileName string) string {
	packageName = strings.TrimPrefix(packageName, m.Name)
	return filepath.Join(m.Path, packageName, fileName)
}

// Returns the package path from a given file path.
// E.g. if the root module path is /abc/xyz/somemodule, the module name is somemodule
// and the file path is "/abc/xyz/somemodule/internal/xyz/file.go" then
// the resulting package path would be somemodule/internal/xyz
// Note that for this to work the given file path must have the root path of the module as a prefix
func (m Module) PackagePathFromFilePath(filePath string) (string, error) {
	filePath = filepath.Clean(filePath)
	if filepath.Ext(filePath) != "" {
		//remove filename if there is one
		filePath = filepath.Dir(filePath)
	}
	if !strings.HasPrefix(filePath, m.Path) {
		return "", errors.New(fmt.Sprintf("cannot compute package path, file is outside module directory: %v", filePath))
	}

	pp, err := filepath.Rel(m.Path, filePath)
	if err != nil {
		return "", errors.New(fmt.Sprintf("cannot compute package path: %v", err))
	}

	return path.Join(m.Name, pp), nil
}
