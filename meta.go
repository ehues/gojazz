package main

import (
	"encoding/gob"
	"os"
	"path/filepath"
)

const (
	metadataFileName = ".jazzmeta"
)

// TODO convert into a smaller object
type metaObject struct {
	Path         string
	ItemId       string
	StateId      string
	LastModified int64
	Size         int64
	Hash         string
	ComponentId  string
}

type metaData struct {
	pathMap       map[string]metaObject
	componentEtag map[string]string
	isstream      bool
	ccmBaseUrl    string
	workspaceId   string
	projectName   string
	userId        string

	inited    bool
	storeMeta chan metaObject
	sync      chan int
}

func newMetaData() *metaData {
	metadata := &metaData{}

	metadata.pathMap = make(map[string]metaObject)
	metadata.componentEtag = make(map[string]string)

	metadata.inited = false

	return metadata
}

func (metadata *metaData) load(path string) error {
	file, err := os.Open(path)
	if err == nil {
		defer file.Close()

		decoder := gob.NewDecoder(file)
		err = decoder.Decode(&metadata.isstream)
		err = decoder.Decode(&metadata.ccmBaseUrl)
		err = decoder.Decode(&metadata.workspaceId)
		err = decoder.Decode(&metadata.projectName)
		err = decoder.Decode(&metadata.userId)
		err = decoder.Decode(&metadata.pathMap)
		err = decoder.Decode(&metadata.componentEtag)
	}

	return err
}

func (metadata *metaData) save(path string) error {
	if metadata.inited {
		// Synchronize first and then write out the metadata
		metadata.sync <- 1
	}

	file, err := os.Create(path)
	if err == nil {
		defer file.Close()

		encoder := gob.NewEncoder(file)
		err = encoder.Encode(&metadata.isstream)
		err = encoder.Encode(&metadata.ccmBaseUrl)
		err = encoder.Encode(&metadata.workspaceId)
		err = encoder.Encode(&metadata.projectName)
		err = encoder.Encode(&metadata.userId)
		err = encoder.Encode(&metadata.pathMap)
		err = encoder.Encode(&metadata.componentEtag)
	}

	return err
}

func (metadata *metaData) initConcurrentWrite() {
	metadata.storeMeta = make(chan metaObject)
	metadata.sync = make(chan int)

	metadata.inited = true

	go func() {
		for {
			select {
			case data := <-metadata.storeMeta:
				metadata.pathMap[data.Path] = data
			case <-metadata.sync:
				// Shutdown after synchronizing
				metadata.inited = false
				return
			}
		}
	}()
}

func (metadata *metaData) put(obj metaObject, sandboxpath string) {
	if !metadata.inited {
		panic("Metadata is not initialized for concurrent write, call initConcurentWrite first")
	}

	// Reduce the path of the metadata object using the sandbox path
	//  this will dramatically decrease the size of the metadata
	relpath, err := filepath.Rel(sandboxpath, obj.Path)

	if err != nil {
		panic(err)
	}

	obj.Path = relpath

	metadata.storeMeta <- obj
}

func (metadata *metaData) simplePut(obj metaObject, sandboxpath string) {
	// Reduce the path of the metadata object using the sandbox path
	//  this will dramatically decrease the size of the metadata
	relpath, err := filepath.Rel(sandboxpath, obj.Path)

	if err != nil {
		panic(err)
	}

	obj.Path = relpath

	metadata.pathMap[relpath] = obj
}

func (metadata *metaData) get(path string, sandboxpath string) (metaObject, bool) {
	// All metadata lookups are based on relative path
	relpath, err := filepath.Rel(sandboxpath, path)

	if err != nil {
		panic(err)
	}

	meta, hit := metadata.pathMap[relpath]

	// This may be an zero (empty) metadata object (ie. a miss on the metadata map)
	//   Don't do any manipulation of the path
	if hit {
		meta.Path = filepath.Join(sandboxpath, meta.Path)
	}

	return meta, hit
}
