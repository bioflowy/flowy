package internal

import (
	"path/filepath"
)

type DataMap map[string]interface{}
type FileOrDirectory interface {
	GetClass() string
	IsFile() bool
	IsDirectory() bool
	HasLocation() bool
	GetLocation() string
	SetLocation(string)
	ClearPath()
	HasPath() bool
	GetPath() string
	SetPath(string)
	HasBasename() bool
	GetBasename() string
	SetBasename(string)
}
type Directory interface {
	GetClass() string
	IsFile() bool
	IsDirectory() bool
	HasLocation() bool
	GetLocation() string
	SetLocation(string)
	ClearPath()
	HasPath() bool
	GetPath() string
	SetPath(string)
	GetListing() []map[string]interface{}
	SetListing([]map[string]interface{})
	HasBasename() bool
	GetBasename() string
	SetBasename(string)
}

func (d DataMap) GetStringPtr(key string) *string {
	val, ok := d[key]
	if !ok {
		return nil
	}
	str, ok := val.(string)
	if !ok {
		return nil
	}
	return &str
}
func (d DataMap) GetFloat32Ptr(key string) *float32 {
	val, ok := d[key]
	if !ok {
		return nil
	}
	str, ok := val.(float32)
	if !ok {
		return nil
	}
	return &str
}

func NewDirectory(location string, path *string) map[string]interface{} {
	dir := map[string]interface{}{
		"class":    "Directory",
		"location": location,
		"basename": filepath.Base(location),
	}
	if path != nil {
		dir["path"] = *path
	}
	return dir
}
func splitext(path string) (root, ext string) {
	ext = filepath.Ext(path)
	root = path[:len(path)-len(ext)]
	return
}

func (o DataMap) SetListing(listing []map[string]interface{}) {
	o["listing"] = listing
}
func (o DataMap) GetListing() []map[string]interface{} {
	l, exists := o["listing"]
	if !exists {
		return nil
	}
	list, ok := l.([]map[string]interface{})
	if ok {
		return list
	}
	class, ok := l.([]interface{})
	if ok {
		listing := make([]map[string]interface{}, len(class))
		for i, val := range class {
			switch v := val.(type) {
			case map[string]interface{}:
				listing[i] = v
			case DataMap:
				listing[i] = v
			}
		}
		return listing
	}
	panic("listing is not a list of maps")
}

type File interface {
	GetClass() string
	IsFile() bool
	IsDirectory() bool
	HasLocation() bool
	GetLocation() string
	SetLocation(string)
	HasPath() bool
	GetPath() string
	SetPath(string)
	ClearPath()
	HasBasename() bool
	GetBasename() string
	SetBasename(string)
	GetDirname() string
	SetDirname(string)
	GetNameroot() string
	SetNameroot(string)
	HasChecksum() bool
	GetChecksum() string
	SetChecksum(string)
	GetSize() int64
	SetSize(int64)
	GetContent() string
	SetContent(string)
	GetWritable() bool
	SetWritable(bool)
	GetSecondaryFiles() []map[string]interface{}
	SetSecondaryFiles([]map[string]interface{})
}

func NewFile(location string, path *string) map[string]interface{} {
	decodedBasename := filepath.Base(location)
	nameroot, nameext := splitext(decodedBasename)
	file := map[string]interface{}{
		"class":    "File",
		"location": location,
		"basename": decodedBasename,
		"nameroot": nameroot,
		"nameext":  nameext,
	}
	if path != nil {
		DataMap(file).SetPath(*path)
	}
	return file
}

func (o DataMap) GetClass() string {
	l, exists := o["class"]
	if !exists {
		return ""
	}
	class, ok := l.(string)
	if ok {
		return class
	} else {
		return ""
	}
}
func (o DataMap) SetClass(className string) {
	o["class"] = className
}
func (o DataMap) IsFile() bool {
	return o.GetClass() == "File"
}
func (o DataMap) IsDirectory() bool {
	return o.GetClass() == "Directory"
}

// GetLocation returns the Location field value if set, zero value otherwise.
func (o DataMap) HasLocation() bool {
	_, exists := o["location"]
	return exists
}
func (o DataMap) GetLocation() string {
	l, exists := o["location"]
	if !exists {
		return ""
	}
	location, ok := l.(string)
	if ok {
		return location
	} else {
		return ""
	}
}
func (o DataMap) SetLocation(location string) {
	o["location"] = location
}

// GetLocation returns the Location field value if set, zero value otherwise.
func (o DataMap) HasPath() bool {
	_, exists := o["path"]
	return exists
}
func (o DataMap) GetPath() string {
	p, exists := o["path"]
	if !exists {
		return ""
	}
	value, ok := p.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) ClearPath() {
	delete(o, "path")
}
func (o DataMap) ClearDirName() {
	delete(o, "dirname")
}
func (o DataMap) SetPath(path string) {
	o["path"] = path
}
func (o DataMap) HasBasename() bool {
	_, exists := o["basename"]
	return exists
}
func (o DataMap) GetBasename() string {
	v, exists := o["basename"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetBasename(path string) {
	o["basename"] = path
}
func (o DataMap) GetDirname() string {
	v, exists := o["dirname"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetDirname(path string) {
	o["dirname"] = path
}
func (o DataMap) GetNameroot() string {
	v, exists := o["nameroot"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetNameroot(path string) {
	o["nameroot"] = path
}
func (o DataMap) GetNameext() string {
	v, exists := o["nameext"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetNameext(path string) {
	o["nameext"] = path
}
func (o DataMap) HasChecksum() bool {
	_, exists := o["checksum"]
	return exists
}

func (o DataMap) GetChecksum() string {
	v, exists := o["checksum"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetChecksum(path string) {
	o["checksum"] = path
}
func (o DataMap) SetContent(path string) {
	o["contents"] = path
}
func (o DataMap) GetContent() string {
	v, exists := o["contents"]
	if !exists {
		return ""
	}
	value, ok := v.(string)
	if ok {
		return value
	} else {
		return ""
	}
}
func (o DataMap) SetSize(path int64) {
	o["size"] = path
}
func (o DataMap) GetSize() int64 {
	v, exists := o["size"]
	if !exists {
		return -1
	}
	value, ok := v.(int64)
	if ok {
		return value
	} else {
		return -1
	}
}
func (o DataMap) SetWritable(path bool) {
	o["writable"] = path
}
func (o DataMap) GetWritable() bool {
	v, exists := o["writable"]
	if !exists {
		return false
	}
	value, ok := v.(bool)
	if ok {
		return value
	} else {
		return false
	}
}
func (o DataMap) SetSecondaryFiles(secondaryFiles []map[string]interface{}) {
	o["secondaryFiles"] = secondaryFiles
}
func (o DataMap) GetSecondaryFiles() []map[string]interface{} {
	v, exists := o["secondaryFiles"]
	if !exists {
		return nil
	}
	files, ok := v.([]interface{})
	if ok {
		secondaryFiles := make([]map[string]interface{}, len(files))
		for i, val := range files {
			secondaryFiles[i] = val.(map[string]interface{})
		}
		return secondaryFiles
	}
	files2, ok := v.([]map[string]interface{})
	if ok {
		return files2
	}
	return nil
}
func IsFileOrDirectory(arg map[string]interface{}) bool {
	class, ok := arg["class"]
	if !ok {
		return false
	}
	return class == "File" || class == "Directory"
}
func IsFile(arg map[string]interface{}) bool {
	class, ok := arg["class"]
	if !ok {
		return false
	}
	return class == "File"
}
func IsDirectory(arg map[string]interface{}) bool {
	class, ok := arg["class"]
	if !ok {
		return false
	}
	return class == "Directory"
}
func VisitFileOrDirectory(arg interface{}, visitInside bool, visitFunc func(FileOrDirectory) error) error {
	if arg == nil {
		return nil
	}
	switch v := arg.(type) {
	case map[string]interface{}:
		if IsFileOrDirectory(v) {
			fd := DataMap(v)
			err := visitFunc(fd)
			if err != nil {
				return err
			}
			if visitInside {
				if fd.IsFile() {
					VisitFileOrDirectory(fd.GetSecondaryFiles(), visitInside, visitFunc)
				} else if fd.IsDirectory() {
					VisitFileOrDirectory(fd.GetListing(), visitInside, visitFunc)
				}
			}
		} else {
			for _, val := range v {
				err := VisitFileOrDirectory(val, visitInside, visitFunc)
				if err != nil {
					return err
				}
			}
		}
	case []interface{}:
		for _, val := range v {
			err := VisitFileOrDirectory(val, visitInside, visitFunc)
			if err != nil {
				return err
			}
		}
	case []map[string]interface{}:
		for _, val := range v {
			err := VisitFileOrDirectory(val, visitInside, visitFunc)
			if err != nil {
				return err
			}
		}
	case []FileOrDirectory:
		panic("not implemented")
	case []File:
		panic("not implemented")
	default:
	}
	return nil
}
func VisitFile(arg interface{}, visitInside bool, visitFunc func(File) error) error {
	return VisitFileOrDirectory(arg, visitInside, func(val FileOrDirectory) error {
		if val.IsFile() {
			return visitFunc(val.(File))
		}
		return nil
	})
}
func VisitDirectory(arg interface{}, visitInside bool, visitFunc func(Directory) error) error {
	return VisitFileOrDirectory(arg, visitInside, func(val FileOrDirectory) error {
		if val.IsDirectory() {
			return visitFunc(val.(Directory))
		}
		return nil
	})
}
