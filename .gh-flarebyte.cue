package ghflarebyte

project: {
  org:  "flarebyte"
  repo: "quick-quack-quest"
}

sync: {
  mode: "push"
}

repository: {
  description:   "Go CLI to validate datasets and run parameterized DuckDB queries from CUE config"
  defaultBranch: "main"
  homepage:      "https://github.com/flarebyte/quick-quack-quest"
  visibility:    "public"
  template:      false
  topics: [
    "flarebyte",
    "go",
    "cobra-cli",
    "duckdb",
    "cue",
    "data-validation",
  ]
  labels: [
    {
      name:        "bug"
      color:       "d73a4a"
      description: "Something is not working"
    },
    {
      name:        "documentation"
      color:       "0075ca"
      description: "Improvements or additions to documentation"
    },
    {
      name:        "duplicate"
      color:       "cfd3d7"
      description: "This issue or pull request already exists"
    },
    {
      name:        "enhancement"
      color:       "a2eeef"
      description: "New feature or request"
    },
    {
      name:        "good first issue"
      color:       "7057ff"
      description: "Good for newcomers"
    },
    {
      name:        "help wanted"
      color:       "008672"
      description: "Extra attention is needed"
    },
    {
      name:        "invalid"
      color:       "e4e669"
      description: "This does not seem right"
    },
    {
      name:        "question"
      color:       "d876e3"
      description: "Further information is requested"
    },
    {
      name:        "wontfix"
      color:       "ffffff"
      description: "This will not be worked on"
    },
  ]
  features: {
    issues:                       true
    wiki:                         false
    projects:                     false
    discussions:                  false
    autoMerge:                    true
    mergeCommit:                  false
    rebaseMerge:                  false
    squashMerge:                  true
    squashMergeCommitMessage:     "pr-title"
    deleteBranchOnMerge:          true
    allowForking:                 false
    allowUpdateBranch:            false
    advancedSecurity:             true
    secretScanning:               true
    secretScanningPushProtection: true
  }
}

build: {
  language:             "go"
  mode:                 "binary"
  outputDir:            "build"
  checksumFile:         "build/checksums.txt"
  artifactTargetSuffix: true
  targets: [
    "darwin-arm64",
    "linux-amd64",
  ]
}

format: {
  env: {
    GOTOOLCHAIN: "local"
    GOCACHE:     ".gocache"
    GOMODCACHE:  ".gomodcache"
  }
}

lint: {
  env: {
    GOTOOLCHAIN: "local"
    GOCACHE:     ".gocache"
    GOMODCACHE:  ".gomodcache"
  }
}

test: {
  env: {
    GOTOOLCHAIN: "local"
    GOCACHE:     ".gocache"
    GOMODCACHE:  ".gomodcache"
  }
}

release: {
  versionSource:    "main.project.yaml"
  tagPrefix:        "v"
  notesMode:        "generate-notes"
  includeArtifacts: true
  artifactDir:      "build"
  includeChecksums: true
}
