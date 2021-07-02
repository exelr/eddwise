package main

import (
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/exelr/eddwise/internal/eddgen"
	"golang.org/x/mod/modfile"
)

var (
	version string
	commit  string
	date    string
	builtBy string
)

var (
	designPath        string
	mode              string
	genCodeServerPath = ""
	genCodeClientPath = ""
	moduleName        = GetModuleName()
)

func printVersion() {
	var com = commit
	if len(com) > 7 {
		com = com[:7]
	}
	fmt.Printf("Edd WiSe version %s\n", version)
	fmt.Printf("Released on %s (%s)\n", date, com)
}
func init() {

	flag.Usage = func() {
		fmt.Printf("Usage: %s [flags] path/to/design mode\n\n", os.Args[0])
		fmt.Printf("\tmode:\n")
		fmt.Printf("\t - 'gen' to generate both client and server definitions\n")
		fmt.Printf("\t - 'skeleton' to generate both client and server empty integrations\n\n")
		fmt.Printf("Available flags:\n")
		flag.PrintDefaults()
	}
	flag.StringVar(&genCodeServerPath, "spath", "", "specify the directory to save server autogenerated code (or - to skip generation). Default to 'gen' for gen mode and 'cmd' for skeleton mode")
	flag.StringVar(&genCodeClientPath, "cpath", "", "specify the directory to save client autogenerated code (or - to skip generation). Default to 'gen' for gen mode and 'web' for skeleton mode")
	flag.StringVar(&moduleName, "mod", moduleName, "the module to be used in skeleton project, default is from go.mod in current working dir")
	var v = flag.Bool("v", false, "print version and exit")
	flag.Parse()
	if v != nil && *v {
		printVersion()
		os.Exit(0)
	}
	if flag.NArg() != 2 {
		flag.Usage()
		os.Exit(1)
	}
	designPath = flag.Arg(0)
	mode = flag.Arg(1)
	if mode != "skeleton" && mode != "gen" {
		flag.Usage()
		os.Exit(1)
	}
	if len(genCodeServerPath) == 0 {
		switch mode {
		case "gen":
			genCodeServerPath = "gen"
		case "skeleton":
			genCodeServerPath = "cmd"
		}
	}
	if len(genCodeClientPath) == 0 {
		switch mode {
		case "gen":
			genCodeClientPath = "gen"
		case "skeleton":
			genCodeClientPath = "web"
		}
	}

}

func GetModuleName() string {
	goModBytes, err := ioutil.ReadFile("go.mod")
	if err != nil {
		return ""
	}

	return modfile.ModulePath(goModBytes)
}

func gen() {
	var filesName, err = getValidFilesInDesignPath()
	if err != nil {
		log.Fatalln(err)
	}

	design, err := eddgen.ParseAndValidateYamls(moduleName, filesName...)
	if err != nil {
		log.Fatalln(err)
	}

	if genCodeServerPath != "-" {
		{
			var serverPath = genCodeServerPath + "/" + design.Name
			if err := os.MkdirAll(serverPath, os.ModePerm); err != nil && os.IsExist(err) {
				log.Fatalln("unable to create server path for code generation:", err)
			}
			var fileName = serverPath + "/channel.go"
			fmt.Println(fileName)
			serverWriter, err := os.Create(fileName)
			if err != nil {
				log.Fatalln("unable to write server file:", err)
			}

			if err := design.GenerateServer(serverWriter); err != nil {
				log.Fatalln(err)
			}
			_ = serverWriter.Close()
		}
		{
			var serverPath = genCodeServerPath + "/" + design.Name + "/behave"
			if err := os.MkdirAll(serverPath, os.ModePerm); err != nil && os.IsExist(err) {
				log.Fatalln("unable to create server test path for code generation:", err)
			}
			var fileName = serverPath + "/channel.go"
			fmt.Println(fileName)
			serverTestWriter, err := os.Create(fileName)
			if err != nil {
				log.Fatalln("unable to write server test file:", err)
			}

			if err := design.GenerateServerTest(serverTestWriter); err != nil {
				log.Fatalln(err)
			}
			_ = serverTestWriter.Close()
		}
	}

	if genCodeClientPath != "-" {
		var clientPath = genCodeClientPath + "/" + design.Name
		if err := os.MkdirAll(clientPath, os.ModePerm); err != nil && os.IsExist(err) {
			log.Fatalln("unable to create client path for code generation:", err)
		}

		var fileName = clientPath + "/channel.js"
		fmt.Println(fileName)
		clientWriter, err := os.Create(fileName)
		if err != nil {
			log.Fatalln("unable to write client file:", err)
		}

		if err := design.GenerateClient(clientWriter); err != nil {
			log.Fatalln(err)
		}
		_ = clientWriter.Close()
	}
}

func getValidFilesInDesignPath() ([]string, error) {
	var filesName []string
	var err = filepath.Walk(designPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		var ext string
		if len(path) > 7 {
			ext = path[len(path)-4:]
		}
		if ext == ".yml" {
			filesName = append(filesName, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("unable to open directory %s: %s\n", designPath, err)
	}

	if len(filesName) == 0 {
		return nil, fmt.Errorf("no .yml files found in specified design path\n")
	}
	return filesName, nil
}

func skeleton() {
	var filesName, err = getValidFilesInDesignPath()
	if err != nil {
		log.Fatalln(err)
	}

	if len(moduleName) == 0 {
		log.Fatalln("you have to provide a module name to generate skeleton, use go.mod file or -mod flag")
	}

	design, err := eddgen.ParseAndValidateYamls(moduleName, filesName...)
	if err != nil {
		log.Fatalln(err)
	}

	if genCodeServerPath != "-" {
		var serverPath = genCodeServerPath + "/" + design.Name
		if err := os.MkdirAll(serverPath, os.ModePerm); err != nil && os.IsExist(err) {
			log.Fatalln("unable to create server path for skeleton generation:", err)
		}
		var fileName = serverPath + "/main.go"
		fmt.Println(fileName)
		serverWriter, err := os.Create(fileName)
		if err != nil {
			log.Fatalln("unable to write server file:", err)
		}

		if err := design.SkeletonServer(serverWriter); err != nil {
			log.Fatalln(err)
		}
		_ = serverWriter.Close()
	}

	if genCodeClientPath != "-" {
		var clientPath = genCodeClientPath + "/" + design.Name
		if err := os.MkdirAll(clientPath, os.ModePerm); err != nil && os.IsExist(err) {
			log.Fatalln("unable to create client path for skeleton generation:", err)
		}

		var fileName = clientPath + "/app.html"
		fmt.Println(fileName)
		clientWriter, err := os.Create(fileName)
		if err != nil {
			log.Fatalln("unable to write client file:", err)
		}

		if err := design.SkeletonClient(clientWriter); err != nil {
			log.Fatalln(err)
		}
		_ = clientWriter.Close()
	}

}

func main() {

	switch mode {
	case "gen":
		gen()
	case "skeleton":
		skeleton()
	}

}
