// Package eooaoo computes auditable, point-based Extent of Occurrence (EOO)
// and Area of Occupancy (AOO) metrics using GDAL/PROJ.
//
// Calculations normalize input coordinates to WGS84, preserve record provenance,
// project to an explicit equal-area analysis CRS, compute the convex-hull EOO,
// and evaluate occupied square-grid cells for AOO. The default AOO cell side is
// 2 km. Results record software, GDAL and PROJ versions, projection definitions,
// grid metadata, warnings and an input hash for reproducibility.
//
// Two analysis strategies are available: ProjectionLocalLAEA, a local Lambert
// Azimuthal Equal Area projection centred on the distribution, and
// ProjectionIUCNCEA, a World Cylindrical Equal Area compatibility-oriented mode
// matching the projection documented by the IUCN EOO Calculator materials.
// ProjectionIUCNCEA does not imply IUCN certification.
//
// The package computes spatial metrics only. It does not assign IUCN Red List
// categories and does not decide whether occurrence records are biologically
// suitable for an assessment.
package eooaoo
