package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "util.go",
        "util_unix.go",
        "util_unsupported.go",
        "util_windows.go",
    ],
    importmap = "kubevirt.io/kubevirt/vendor/k8s.io/kubernetes/pkg/kubelet/util",
    importpath = "k8s.io/kubernetes/pkg/kubelet/util",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
    ] + select({
        "@io_bazel_rules_go//go/platform:android": [
            "//staging/src/github.com/golang/glog:go_default_library",
            "//vendor/golang.org/x/sys/unix:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:darwin": [
            "//staging/src/github.com/golang/glog:go_default_library",
            "//vendor/golang.org/x/sys/unix:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:freebsd": [
            "//staging/src/github.com/golang/glog:go_default_library",
            "//vendor/golang.org/x/sys/unix:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:ios": [
            "//staging/src/github.com/golang/glog:go_default_library",
            "//vendor/golang.org/x/sys/unix:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//staging/src/github.com/golang/glog:go_default_library",
            "//vendor/golang.org/x/sys/unix:go_default_library",
        ],
        "//conditions:default": [],
    }),
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [
        ":package-srcs",
        "//pkg/kubelet/util/cache:all-srcs",
        "//pkg/kubelet/util/format:all-srcs",
        "//pkg/kubelet/util/ioutils:all-srcs",
        "//pkg/kubelet/util/manager:all-srcs",
        "//pkg/kubelet/util/pluginwatcher:all-srcs",
        "//pkg/kubelet/util/queue:all-srcs",
        "//pkg/kubelet/util/sliceutils:all-srcs",
        "//pkg/kubelet/util/store:all-srcs",
    ],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
