package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type logWriter int

// Skipped don't copy these files
var Skipped = map[string]int{".DS_Store": 1}

func (x logWriter) Write(bytes []byte) (int, error) {
	path := fmt.Sprintf("./%s.log", time.Now().Format("20060102"))
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	defer file.Close()
	if err != nil {
		panic(fmt.Sprintf("os.OpenFile(): %v", err))
	}
	file.WriteString(time.Now().Format("15:04:05") + " " + string(bytes))

	return fmt.Print(string(bytes))
}

func main() {

	// config log
	log.SetFlags(0)
	log.SetOutput(new(logWriter))
	log.Println("Application start")

	// get application settings
	var c config
	appsettings, err := ioutil.ReadFile("./appsettings.json")
	if err != nil {
		fmt.Println("ioutil.ReadFile()", err)
		return
	}

	err = json.Unmarshal(appsettings, &c)
	if err != nil {
		fmt.Println("json.Unmarshal()", err)
		return
	}
	log.Println("Source:", c.Source)
	log.Println("Target:", c.Target)

	// confirm user input
	fmt.Printf("Press OK to continue, anything else to quit:")

	scanner := bufio.NewScanner(os.Stdin)
	var text string
	for scanner.Scan() {
		text = scanner.Text()
		break
	}

	if text != "OK" {
		return
	}
	start := time.Now()

	// sync file
	added, deleted := sync(c.Source, c.Target, 0)
	elapsed := time.Since(start)

	log.Printf("finish! total add:%v, del:%v, takes %v.", added, deleted, getElapsedText(elapsed))
	log.Println()
}

func getElapsedText(elapsed time.Duration) string {
	if elapsed.Hours() > 1 {
		return fmt.Sprintf("%.2f hours", elapsed.Hours())
	}
	if elapsed.Minutes() > 1 {
		return fmt.Sprintf("%.2f minutes", elapsed.Minutes())
	}

	return fmt.Sprintf("%.2f seconds", elapsed.Seconds())
}

func sync(source, target string, level int) (int, int) {
	start := time.Now()

	addCount := 0
	delCount := 0
	tab := ""
	if level > 0 {
		tab = strings.Repeat("-", int(level*4)) + " "
	}

	// 1.get source files & dirs
	sourceFiles, sourceDirs := getFiles(source)
	if !exists(source) {
		return 0, 0
	}

	// 2. get target files
	if !exists(target) {
		os.Mkdir(target, os.ModePerm)
	}
	targetFiles, targetDirs := getFiles(target)

	// 3. compare source files and target files
	added, deleted := getDiff(sourceFiles, targetFiles)
	_, deletedDirs := getDiff(sourceDirs, targetDirs)

	addCount += copyFiles(added, source, target)
	delCount += delFiles(deleted, target)

	delDirsCount := delFiles(deletedDirs, target)

	elapsed := time.Since(start)
	log.Printf("%v%v add: %v, del: %v files, %v dirs, takes %v .\n", tab, filepath.Base(target), addCount, delCount, delDirsCount, getElapsedText(elapsed))

	// 5. recursion all dirs
	for _, dir := range sourceDirs {
		name := dir.Name()
		source := path.Join(source, name)
		target := path.Join(target, name)
		a, d := sync(source, target, level+1)
		addCount += a
		delCount += d
	}

	return addCount, delCount
}

// copy files from source to target
func copyFiles(added []string, source, target string) int {

	BUFFERSIZE := 1024 * 1024 * 20 // 20MB
	i := 0

	for _, f := range added {
		sourcePath := path.Join(source, f)
		targetPath := path.Join(target, f)

		buf := make([]byte, BUFFERSIZE)

		source, err := os.Open(sourcePath)
		if err != nil {
			log.Println("copyFiles os.Open()", err)
			return 0
		}
		defer source.Close()

		target, err := os.Create(targetPath)
		if err != nil {
			return 0
		}
		defer target.Close()

		for {
			n, err := source.Read(buf)
			if err != nil && err != io.EOF {
				log.Println("copyFiles source.Read()", err)
				break
			}
			if n == 0 {
				i++
				break
			}
			if _, err := target.Write(buf[:n]); err != nil {
				log.Println("copyFiles source.Read()", err)
				break
			}
		}
	}
	return i
}

// delete files in target
func delFiles(deleted []string, target string) int {
	i := 0

	for _, file := range deleted {
		filePath := path.Join(target, file)
		err := os.RemoveAll(filePath)
		if err != nil {
			log.Println("os.Remove", err)
			continue
		}
		i++
	}
	return i
}

// get difference between source and target directory
func getDiff(sourceFiles, targetFiles []os.FileInfo) ([]string, []string) {
	var m = make(map[string]int)
	added := []string{}
	deleted := []string{}

	for _, f := range sourceFiles {
		m[f.Name()] = 1 // added
	}

	for _, f := range targetFiles {
		if _, ok := m[f.Name()]; ok {
			m[f.Name()] = 0 // equal
		} else {
			m[f.Name()] = -1 // deleted
		}
	}

	for k, v := range m {
		if v == 1 {
			added = append(added, k)
		} else if v == -1 {
			deleted = append(deleted, k)
		}
	}
	return added, deleted
}

// check file exists
func exists(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// getFiles: get file list and dir list
func getFiles(path string) ([]os.FileInfo, []os.FileInfo) {

	dir, err := os.Open(path)
	defer dir.Close()

	if err != nil {
		log.Println("os.Open()", err)
		return nil, nil
	}

	files := []os.FileInfo{}
	dirs := []os.FileInfo{}

	list, err := dir.Readdir(-1)
	if err != nil {
		log.Println("dir.Readdir()", err)
		return nil, nil
	}

	for _, f := range list {
		if f.IsDir() {
			dirs = append(dirs, f)
		} else if f.Mode().IsRegular() {
			if _, ok := Skipped[f.Name()]; !ok {
				files = append(files, f)
			}
		}
	}
	return files, dirs
}

type config struct {
	Source string `json:"source"`
	Target string `json:"target"`
}
