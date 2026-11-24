module github.com/Lzeutschler/commander_ai

go 1.25.8

require github.com/adams-shaun/gorge v0.0.0

// The engine is pinned as a git submodule (see README.md): its card tables are
// generated into the checkout, so it cannot be consumed as a plain module.
replace github.com/adams-shaun/gorge => ./third_party/gorge
