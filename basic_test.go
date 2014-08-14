package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

var (
	userId = os.Getenv("DOS_USERID")

	testContents = []string{
		"README.md", "project.json", "bigFile.txt", ".jazzignore",
		".cfignore", "folder", "filename(with)[chars$]^that.must-be-escaped", "bin",
		"bin/mybinary.so", "filename(with)[chars$]^that.must-be-escaped/test.java",
		"folder/file.exe", "folder/file1.txt", "folder/file2.jsp", "folder/file3.jar",
		"folder/filename(with)[chars$]^that.must-be-escaped",
	}

	testContentsWithoutIgnoredStuff = []string{
		"README.md", "project.json", "bigFile.txt", ".jazzignore",
		".cfignore", "folder", "filename(with)[chars$]^that.must-be-escaped",
		"filename(with)[chars$]^that.must-be-escaped/test.java",
		"folder/file1.txt", "folder/file2.jsp", "folder/file3.jar",
		"folder/filename(with)[chars$]^that.must-be-escaped",
	}

	testContentsAlternate = []string{
		"project.json", "bigFile.txt", ".jazzignore",
		".cfignore", "folder", "filename(with)[chars$]^that.must-be-escaped", "bin",
		"bin/mybinary.so", "filename(with)[chars$]^that.must-be-escaped/test.java",
		"folder/file.exe", "folder/file2.jsp", "folder/file3.jar",
		"folder/filename(with)[chars$]^that.must-be-escaped",
		"alternateFile.txt", "alternateFolder", "alternateFolder/anotherAlternateFolder",
		"alternateFolder/anotherAlternateFile.txt",
	}
)

func init() {
	if userId == "" || password == "" {
		fmt.Printf("Please use the test.sh script to run the tests.\n")
		os.Exit(1)
	}
}

func TestBasicStreamLoad(t *testing.T) {
	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", "sirnewton | gojazz-test", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	// Verify that specific files show up
	for _, file := range testContents {
		p := filepath.Join(sandbox1, file)
		s, _ := os.Stat(p)
		if s == nil {
			t.Error("File not found in sandbox: %v", p)
		}
	}
}

func TestStreamLoadOnExistingFiles(t *testing.T) {
	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	deletemePath := filepath.Join(sandbox1, "deleteme.txt")
	deleteme, err := os.Create(deletemePath)
	if err != nil {
		panic(err)
	}
	defer deleteme.Close()
	_, err = deleteme.Write([]byte("delete this"))
	if err != nil {
		panic(err)
	}
	deleteme.Close()

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", "sirnewton | gojazz-test", "-sandbox=" + sandbox1, "-force=true"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	s, _ := os.Stat(deletemePath)
	if s != nil {
		t.Fail()
	}

	// TODO maybe someday we should back up what was in the sandbox before the first load
	// Check if the deleteme is backed up in the backup folder
	//s, _ = os.Stat(filepath.Join(sandbox1, backupFolder, "deleteme.txt"))
	//if s == nil {
	//	t.Fail()
	//}
}

func TestLoadAndClobberChanges(t *testing.T) {
	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", "sirnewton | gojazz-test", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	// Make adds and mods to the files
	for _, file := range testContentsWithoutIgnoredStuff {
		path := filepath.Join(sandbox1, file)
		s, _ := os.Stat(path)
		if s == nil {
			t.Error("File not found in sandbox: %v", path)
		}

		if s.IsDir() {
			deleteMe, err := os.Create(filepath.Join(path, "deleteMe.txt"))
			if err != nil {
				panic(err)
			}
			defer deleteMe.Close()
			_, err = deleteMe.Write([]byte("test contents"))
			if err != nil {
				panic(err)
			}

			err = os.Mkdir(filepath.Join(path, "deleteMe"), 0700)
			if err != nil {
				panic(err)
			}
		} else {
			modFile, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				panic(err)
			}
			defer modFile.Close()

			_, err = modFile.Write([]byte("new contents123"))
			if err != nil {
				panic(err)
			}
		}
	}

	os.Args = []string{"load", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	status, err := scmStatus(sandbox1, NO_COPY)
	if err != nil {
		t.Fatalf("%v", err.Error())
	}
	if !status.unchanged() {
		t.Fail()
	}

	// Check that all of the files and folders made their way into the backup
	for _, file := range testContentsWithoutIgnoredStuff {
		path := filepath.Join(sandbox1, backupFolder, file)
		s, _ := os.Stat(path)
		if s == nil {
			t.Fatalf("File not found in backup: %v", path)
		}

		if s.IsDir() {
			s, _ := os.Stat(filepath.Join(path, "deleteMe"))
			if s == nil {
				t.Error("File not found in backup: %v", filepath.Join(path, "deleteMe"))
			}

			s, _ = os.Stat(filepath.Join(path, "deleteMe.txt"))
			if s == nil {
				t.Error("File not found in backup: %v", filepath.Join(path, "deleteMe.txt"))
			}
		}
	}
}

func TestAlternateStreamLoad(t *testing.T) {
	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", "sirnewton | gojazz-test", "-stream=Alternate Stream", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	// Verify that specific files show up
	for _, file := range testContentsAlternate {
		p := filepath.Join(sandbox1, file)
		s, _ := os.Stat(p)
		if s == nil {
			t.Error("File not found in sandbox: %v", p)
		}
	}
}

func TestEmptyStreamLoad(t *testing.T) {
	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", "sirnewton | gojazz-test", "-stream=Empty Stream", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	// Verify that only the jazzMeta file is created in the sandbox
	f, err := os.Open(sandbox1)
	if err != nil {
		panic(err)
	}

	names, err := f.Readdirnames(-1)
	if err != nil {
		panic(err)
	}

	if len(names) != 1 {
		t.Fatalf("Expected 1 file but found %v files", len(names))
	}

	if names[0] != metadataFileName {
		t.Fatalf("Expected only the metadata file but found %v", names[0])
	}
}

func TestSwitchStreams(t *testing.T) {
	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", "sirnewton | gojazz-test", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", "sirnewton | gojazz-test", "-stream=Alternate Stream", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	// Verify that specific files show up
	filesToCheck := []string{
		"project.json", "bigFile.txt", ".jazzignore",
		".cfignore", "folder", "filename(with)[chars$]^that.must-be-escaped", "bin",
		"bin/mybinary.so", "filename(with)[chars$]^that.must-be-escaped/test.java",
		"folder/file.exe", "folder/file2.jsp", "folder/file3.jar",
		"folder/filename(with)[chars$]^that.must-be-escaped",
		"alternateFile.txt", "alternateFolder", "alternateFolder/anotherAlternateFolder",
		"alternateFolder/anotherAlternateFile.txt",
	}
	for _, file := range filesToCheck {
		p := filepath.Join(sandbox1, file)
		s, _ := os.Stat(p)
		if s == nil {
			t.Error("File not found in sandbox: %v", p)
		}
	}

	// Verify that the file from the other stream no longer exists
	filesToCheck = []string{
		"folder/file1.txt", "README.md",
	}
	for _, file := range filesToCheck {
		p := filepath.Join(sandbox1, file)
		s, _ := os.Stat(p)
		if s != nil {
			t.Error("File from the wrong stream found in sandbox: %v", p)
		}
	}

	// Verify that there are no backups since nothing was modified
	s, _ := os.Stat(filepath.Join(sandbox1, backupFolder))
	if s != nil {
		t.Fatalf("Found a backup folder even though no changes were made.")
	}
}

func deleteWorkspace(client *Client, projectName string, workspaceId string) error {
	if client.GetJazzId() == "" {
		return errors.New("Not logged in")
	}

	if workspaceId == "" {
		return errors.New("No workspace ID provided")
	}

	url := path.Join(jazzHubBaseUrl, "/code/jazz/Workspace/", workspaceId, "file", client.GetJazzId()+"-OrionContent", projectName)
	url = strings.Replace(url, ":/", "://", 1)

	request, err := http.NewRequest("DELETE", url, strings.NewReader(`{
	}`))
	if err != nil {
		return err
	}
	addOrionHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	err = waitForOrionResponse(client, resp, nil)
	if err != nil {
		return err
	}

	return nil
}

func deleteProject(client *Client, projectName string) error {
	if client.GetJazzId() == "" {
		return errors.New("Not logged in")
	}

	if projectName == "" {
		return errors.New("No project name provided")
	}

	url := path.Join(jazzHubBaseUrl, "/code/workspace", client.GetJazzId()+"-OrionContent", "project", projectName)
	url = strings.Replace(url, ":/", "://", 1)

	request, err := http.NewRequest("DELETE", url, strings.NewReader(`{
	}`))
	if err != nil {
		return err
	}
	addOrionHeaders(request)

	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	err = waitForOrionResponse(client, resp, nil)
	if err != nil {
		return err
	}

	return nil
}

func cleanWorkspace(projectName string) {
	// Clean up any existing repository workspaces and web IDE projects
	client, err := NewClient(userId, password)
	if err != nil {
		panic(err)
	}

	ccmBaseUrl, err := client.findCcmBaseUrl(projectName)
	if err != nil {
		panic(err)
	}

	workspaceId, err := FindRepositoryWorkspace(client, ccmBaseUrl, projectName+" Workspace")
	if err != nil {
		panic(err)
	}

	if workspaceId != "" {
		err = deleteWorkspace(client, projectName, workspaceId)
		if err != nil {
			panic(err)
		}
	}

	err = deleteProject(client, projectName)
	if err != nil {
		panic(err)
	}
}

func TestLoadWorkspace(t *testing.T) {
	projectName := "sirnewton | gojazz-test2"
	cleanWorkspace(projectName)

	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", projectName, "-sandbox=" + sandbox1, "-workspace=true", "-userId=" + userId}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	defer cleanWorkspace(projectName)

	// Verify that specific files show up
	for _, file := range testContents {
		p := filepath.Join(sandbox1, file)
		s, _ := os.Stat(p)
		if s == nil {
			t.Error("File not found in sandbox: %v", p)
		}
	}
}

func TestWorkspaceLoadAndClobberChanges(t *testing.T) {
	projectName := "sirnewton | gojazz-test2"
	cleanWorkspace(projectName)

	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test2")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", projectName, "-sandbox=" + sandbox1, "-workspace=true", "-userId=" + userId}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	defer cleanWorkspace(projectName)

	// Make adds and mods to the files
	for _, file := range testContentsWithoutIgnoredStuff {
		path := filepath.Join(sandbox1, file)
		s, _ := os.Stat(path)
		if s == nil {
			t.Error("File not found in sandbox: %v", path)
		}

		if s.IsDir() {
			deleteMe, err := os.Create(filepath.Join(path, "deleteMe.txt"))
			if err != nil {
				panic(err)
			}
			defer deleteMe.Close()
			_, err = deleteMe.Write([]byte("test contents"))
			if err != nil {
				panic(err)
			}

			err = os.Mkdir(filepath.Join(path, "deleteMe"), 0700)
			if err != nil {
				panic(err)
			}
		} else {
			modFile, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				panic(err)
			}
			defer modFile.Close()

			_, err = modFile.Write([]byte("new contents123"))
			if err != nil {
				panic(err)
			}
		}
	}

	os.Args = []string{"load", "-sandbox=" + sandbox1}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	status, err := scmStatus(sandbox1, NO_COPY)
	if err != nil {
		t.Fatalf("%v", err.Error())
	}
	if !status.unchanged() {
		t.Fail()
	}

	// Check that all of the files and folders made their way into the backup
	for _, file := range testContentsWithoutIgnoredStuff {
		path := filepath.Join(sandbox1, backupFolder, file)
		s, _ := os.Stat(path)
		if s == nil {
			t.Fatalf("File not found in backup: %v", path)
		}

		if s.IsDir() {
			s, _ := os.Stat(filepath.Join(path, "deleteMe"))
			if s == nil {
				t.Error("File not found in backup: %v", filepath.Join(path, "deleteMe"))
			}

			s, _ = os.Stat(filepath.Join(path, "deleteMe.txt"))
			if s == nil {
				t.Error("File not found in backup: %v", filepath.Join(path, "deleteMe.txt"))
			}
		}
	}
}

func TestLocalChangeDetection(t *testing.T) {
	projectName := "sirnewton | gojazz-test2"
	cleanWorkspace(projectName)

	sandbox1, err := ioutil.TempDir(os.TempDir(), "gojazz-test")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(sandbox1)

	t.Logf("Loading test project into %v\n", sandbox1)
	os.Args = []string{"load", projectName, "-sandbox=" + sandbox1, "-workspace=true", "-userId=" + userId}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	loadOp()

	defer cleanWorkspace(projectName)

	// Make adds, mods and deletes to the files
	rootFolder := filepath.Join(sandbox1, "added")
	err = os.Mkdir(rootFolder, 0700)
	if err != nil {
		panic(err)
	}

	rootFile := filepath.Join(sandbox1, "added.txt")
	f, err := os.Create(rootFile)
	if err != nil {
		panic(err)
	}
	f.Close()

	// Record all of the changes to verify at the end
	numChanges := 2

	for _, file := range testContents {
		path := filepath.Join(sandbox1, file)
		s, _ := os.Stat(path)
		if s == nil {
			t.Error("File not found in sandbox: %v", path)
			continue
		}

		if file == "folder/file1.txt" {
			err := os.Remove(path)
			if err != nil {
				panic(err)
			}
			numChanges += 1
			continue
		}

		ignored, err := IsIgnored(path)
		if err != nil {
			panic(err)
		}

		if s.IsDir() {
			added, err := os.Create(filepath.Join(path, "added.txt"))
			if err != nil {
				panic(err)
			}
			defer added.Close()
			_, err = added.Write([]byte("test contents"))
			if err != nil {
				panic(err)
			}

			err = os.Mkdir(filepath.Join(path, "added"), 0700)
			if err != nil {
				panic(err)
			}

			if !ignored {
				numChanges += 2
			}
		} else {
			modFile, err := os.OpenFile(path, os.O_WRONLY, 0)
			if err != nil {
				panic(err)
			}
			defer modFile.Close()

			_, err = modFile.Write([]byte("new contents123"))
			if err != nil {
				panic(err)
			}

			if !ignored {
				numChanges += 1
			}
		}
	}

	status, err := scmStatus(sandbox1, NO_COPY)
	if err != nil {
		panic(err)
	}
	if status.unchanged() {
		t.Errorf("Status is unchanged even though there are sandbox changes.")
	}

	_, found := status.Added["added"]
	if !found {
		t.Errorf("Added expected but not found: %v", rootFolder)
	}
	_, found = status.Added["added.txt"]
	if !found {
		t.Errorf("Added expected but not found: %v", rootFile)
	}

	numChangesFound := len(status.Added) + len(status.Deleted) + len(status.Modified)
	if numChanges != numChangesFound {
		t.Errorf("Number of changes doesn't match: %v %v", numChanges, numChangesFound)
	}

	// Check that all of the files and folders are reported in the status
	for _, file := range testContentsWithoutIgnoredStuff {
		if file == "folder/file1.txt" {
			_, found := status.Deleted[file]
			if !found {
				t.Errorf("Deletion expected but not found: %v", file)
			}
			continue
		}

		path := filepath.Join(sandbox1, file)
		s, _ := os.Stat(path)
		if s == nil {
			t.Fatalf("File not found in sandbox: %v", path)
		}

		if s.IsDir() {
			newFolder := filepath.Join(file, "added")

			_, found := status.Added[newFolder]
			if !found {
				t.Errorf("Added expected but not found: %v", newFolder)
			}

			newFile := filepath.Join(file, "added.txt")

			_, found = status.Added[newFile]
			if !found {
				t.Errorf("Added expected but not found: %v", newFile)
			}
		} else {
			_, found := status.Modified[file]
			if !found {
				t.Errorf("Modification expected but not found: %v", file)
			}
		}
	}
}