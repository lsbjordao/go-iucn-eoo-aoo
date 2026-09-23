package eooaoo

/*
#cgo pkg-config: proj
#include <proj.h>
static const char* eoo_proj_version(void) { return proj_info().version; }
*/
import "C"

// PROJVersion identifies the loaded native coordinate transformation library.
func PROJVersion() string { return C.GoString(C.eoo_proj_version()) }
