#!/usr/bin/env Rscript
args <- commandArgs(trailingOnly = TRUE)
if (length(args) != 2) {
  stop("usage: setup_conr.R <commit> <library-dir>")
}
revision <- args[[1]]
lib <- normalizePath(args[[2]], mustWork = FALSE)
dir.create(lib, recursive = TRUE, showWarnings = FALSE)
.libPaths(c(lib, .libPaths()))
marker <- file.path(lib, ".conr-revision")
repos <- "https://cloud.r-project.org"

ensure_cran_package <- function(pkg) {
  if (!requireNamespace(pkg, quietly = TRUE)) {
    message("Installing required ConR runtime package ", pkg, " into ", lib)
    install.packages(pkg, lib = lib, repos = repos)
  }
  if (!requireNamespace(pkg, quietly = TRUE)) {
    stop("Required ConR runtime package could not be loaded: ", pkg)
  }
  message(pkg, " ", as.character(utils::packageVersion(pkg)), " available")
}

# ConR lists lwgeom under Suggests, but EOO/AOO code paths used by this
# benchmark may require it at runtime. Check this even when the pinned ConR
# installation itself is already cached.
ensure_cran_package("lwgeom")

if (requireNamespace("ConR", quietly = TRUE) && file.exists(marker)) {
  installed_revision <- trimws(readLines(marker, warn = FALSE)[1])
  if (identical(installed_revision, revision)) {
    message("ConR already installed at pinned revision ", revision)
    message("ConR ", as.character(utils::packageVersion("ConR")))
    quit(status = 0)
  }
}

if (!requireNamespace("remotes", quietly = TRUE)) {
  install.packages("remotes", lib = lib, repos = repos)
}

message("Installing gdauby/ConR at ", revision, " into ", lib)
remotes::install_github(
  paste0("gdauby/ConR@", revision),
  lib = lib,
  dependencies = c("Depends", "Imports", "LinkingTo"),
  upgrade = "never",
  build_vignettes = FALSE
)
writeLines(revision, marker)
message("Installed ConR ", as.character(utils::packageVersion("ConR")))
