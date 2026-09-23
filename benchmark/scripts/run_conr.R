#!/usr/bin/env Rscript
args <- commandArgs(trailingOnly = TRUE)
if (length(args) != 4) {
  stop("usage: run_conr.R <datasets-dir> <output.csv> <library-dir> <revision>")
}
datasets_dir <- args[[1]]
output <- args[[2]]
lib <- normalizePath(args[[3]], mustWork = FALSE)
revision <- args[[4]]
.libPaths(c(lib, .libPaths()))

suppressPackageStartupMessages(library(ConR))
version <- as.character(utils::packageVersion("ConR"))
set.seed(1)

first_numeric <- function(x) {
  if (is.null(x)) return(NA_real_)
  if (is.numeric(x)) return(as.numeric(x[[1]]))
  if (is.data.frame(x)) {
    for (nm in names(x)) {
      if (is.numeric(x[[nm]])) return(as.numeric(x[[nm]][[1]]))
    }
  }
  if (is.list(x)) {
    for (item in x) {
      v <- first_numeric(item)
      if (!is.na(v)) return(v)
    }
  }
  NA_real_
}

safe_value <- function(expr) {
  tryCatch(
    list(value = first_numeric(force(expr)), error = ""),
    error = function(e) list(value = NA_real_, error = conditionMessage(e))
  )
}

files <- sort(list.files(datasets_dir, pattern = "\\.csv$", full.names = TRUE))
rows <- list()
idx <- 1
for (file in files) {
  dat <- read.csv(file, stringsAsFactors = FALSE, check.names = FALSE)
  dataset <- tools::file_path_sans_ext(basename(file))
  tax <- if ("taxon" %in% names(dat)) dat$taxon else rep(dataset, nrow(dat))
  xy <- data.frame(lat = as.numeric(dat$lat), lon = as.numeric(dat$lon), tax = tax)
  unique_coordinates <- nrow(unique(xy[, c("lat", "lon")]))

  aoo <- safe_value(ConR::AOO.computing(
    XY = xy,
    cell_size_AOO = 2,
    nbe.rep.rast.AOO = 0,
    parallel = FALSE,
    show_progress = FALSE,
    export_shp = FALSE,
    proj_type = "cea"
  ))

  eoo_spheroid <- safe_value(ConR::EOO.computing(
    XY = xy,
    export_shp = FALSE,
    method.range = "convex.hull",
    method.less.than3 = "not comp",
    parallel = FALSE,
    show_progress = FALSE,
    proj_type = "cea",
    mode = "spheroid"
  ))

  eoo_planar <- safe_value(ConR::EOO.computing(
    XY = xy,
    export_shp = FALSE,
    method.range = "convex.hull",
    method.less.than3 = "not comp",
    parallel = FALSE,
    show_progress = FALSE,
    proj_type = "cea",
    mode = "planar"
  ))

  settings_base <- "cell_size_AOO=2;nbe.rep.rast.AOO=0;proj_type=cea;set.seed=1"
  for (spec in list(
    list(mode = "default-spheroid", eoo = eoo_spheroid, settings = paste0(settings_base, ";EOO_mode=spheroid")),
    list(mode = "planar-cea", eoo = eoo_planar, settings = paste0(settings_base, ";EOO_mode=planar"))
  )) {
    errors <- Filter(nzchar, c(spec$eoo$error, aoo$error))
    rows[[idx]] <- data.frame(
      dataset = dataset,
      tool = "gdauby/ConR",
      tool_version = version,
      tool_revision = revision,
      mode = spec$mode,
      eoo_raw_km2 = ifelse(is.na(spec$eoo$value), "", format(spec$eoo$value, digits = 17, scientific = FALSE)),
      eoo_assessment_km2 = "",
      aoo_km2 = ifelse(is.na(aoo$value), "", format(aoo$value, digits = 17, scientific = FALSE)),
      occupied_cells = ifelse(is.na(aoo$value), "", format(aoo$value / 4, digits = 17, scientific = FALSE)),
      input_records = nrow(dat),
      unique_coordinates = unique_coordinates,
      error = paste(errors, collapse = " | "),
      settings = spec$settings,
      notes = "ConR pinned GitHub revision; identical occurrence coordinates",
      stringsAsFactors = FALSE
    )
    idx <- idx + 1
  }
}

result <- if (length(rows)) do.call(rbind, rows) else data.frame()
dir.create(dirname(output), recursive = TRUE, showWarnings = FALSE)
write.csv(result, output, row.names = FALSE, na = "")
message("wrote ", nrow(result), " rows to ", output)
