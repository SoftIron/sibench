package main


import "path/filepath"
import "fmt"
import "io/ioutil"
import "os"


/* 
 * An abstract connection backed by a local file system.  
 *
 * It is initialised with a root: a directory under which all operations take place.  This
 * is likely to be the directory where a remote filesystem has been mounted (where the 
 * remote filesystem is backed by the cluster under test), but it could be any dir really.
 *
 * FileConnection is not intented to be used directly, but wrapped in a parent Connection
 * that know how to create and tear-down the mount (such as CephFSConnection).   As such
 * it doesn't have the ususal connection constructor, or a Target() function.
 */
type FileConnection struct {
    root string
}


func (conn *FileConnection) InitFileConnection(root string) {
    fmt.Printf("Initialising FileConnection on %v\n", root)
    conn.root = root
}


func (conn *FileConnection) ListBuckets() ([]string, error) {
    fis, err := ioutil.ReadDir(conn.root)
    if err != nil {
        return nil, err
    }

    var subdirs []string
    for _, fi := range fis {
        if fi.IsDir() {
            subdirs = append(subdirs, fi.Name())
        }
    }

    return subdirs, nil
}


func (conn *FileConnection) CreateBucket(bucket string) error {
    return os.MkdirAll(filepath.Join(conn.root, bucket), 0644)
}


func (conn *FileConnection) DeleteBucket(bucket string) error {
    return os.RemoveAll(filepath.Join(conn.root, bucket))
}


func (conn *FileConnection) ListObjects(bucket string) ([]string, error) {
    fis, err := ioutil.ReadDir(filepath.Join(conn.root, bucket))
    if err != nil {
        return nil, err
    }

    var files []string
    for _, fi := range fis {
        if !fi.IsDir() {
            files = append(files, fi.Name())
        }
    }

    return files, nil
}


func (conn *FileConnection) PutObject(bucket string, key string, contents []byte) error {
    filename := filepath.Join(conn.root, bucket, key)
    return  ioutil.WriteFile(filename, contents, 0644)
}


func (conn *FileConnection) GetObject(bucket string, key string) ([]byte, error) {
    filename := filepath.Join(conn.root, bucket, key)
    return  ioutil.ReadFile(filename)
}


func (conn *FileConnection) Close() {
    // Nothing to do here.
}
