package eooaoo

/*
#cgo pkg-config: gdal
#include <stdlib.h>
#include <cpl_conv.h>
#include <ogr_srs_api.h>

static void eoo_set_proj_search_path(const char *path) {
	const char *paths[2];
	paths[0] = path;
	paths[1] = NULL;
	OSRSetPROJSearchPaths(paths);
}
*/
import "C"

import (
	"os"
	"path/filepath"
	"runtime"
	"unsafe"
)

// configureBundledSpatialData makes portable Windows releases self-contained
// with respect to GDAL and PROJ resource files. Native DLLs are resolved by the
// Windows loader from the executable directory; data directories live under
// share/gdal and share/proj next to the executable.
//
// The locations are published through the native GDAL and PROJ APIs rather than
// the environment alone: on Windows os.Setenv only calls SetEnvironmentVariableW,
// which does not update the C runtime environment that GDAL and PROJ read with
// getenv, so proj.db would not be found from the bundle.
func configureBundledSpatialData() {
	if runtime.GOOS != "windows" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	root := filepath.Dir(exe)
	setIfBundled := func(key, dir string) {
		if os.Getenv(key) != "" {
			return
		}
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			_ = os.Setenv(key, dir)
			setNativeSpatialDataPath(key, dir)
		}
	}
	setIfBundled("GDAL_DATA", filepath.Join(root, "share", "gdal"))
	setIfBundled("PROJ_DATA", filepath.Join(root, "share", "proj"))
}

// setNativeSpatialDataPath tells the loaded GDAL and PROJ libraries where their
// resource files are, independently of the process environment. GDAL applies
// the search path to the PROJ contexts it creates internally.
func setNativeSpatialDataPath(key, dir string) {
	cDir := C.CString(dir)
	defer C.free(unsafe.Pointer(cDir))
	switch key {
	case "GDAL_DATA":
		cKey := C.CString(key)
		defer C.free(unsafe.Pointer(cKey))
		C.CPLSetConfigOption(cKey, cDir)
	case "PROJ_DATA":
		C.eoo_set_proj_search_path(cDir)
	}
}

func init() {
	configureBundledSpatialData()
}
