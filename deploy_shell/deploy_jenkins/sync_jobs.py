#!/usr/bin/env python3
from __future__ import annotations

import argparse
import base64
import copy
import json
import re
import sys
import uuid
import xml.etree.ElementTree as ET
from pathlib import Path, PurePosixPath
from typing import Iterable

DEFAULT_CONFIG_VERSION = 2
DEFAULT_BRANCH_PARAMETER = "origin/master"
DEFAULT_REMOTE_TRIGGER_PREFIX = "deploy-shell-http"
JOB_TYPE_META = {
    "swift_app": {
        "project_dir": "template_swift_app",
        "config_path": "template_swift_app/deploy_config.sh",
        "shell_dir": "deploy_swift_app",
        "view": "📱App",
        "parameter_profile": "swift_app",
        "shell_command_profile": "swift_app",
    },
    "android_app": {
        "project_dir": "template_android_app",
        "config_path": "template_android_app/deploy_config.sh",
        "shell_dir": "deploy_android_app",
        "view": "📱App",
        "parameter_profile": "android_app",
        "shell_command_profile": "android_app",
    },
    "web": {
        "project_dir": "template_web",
        "config_path": "template_web/deploy_config.sh",
        "shell_dir": "deploy_web",
        "view": "🧭Web",
        "parameter_profile": "branch_only",
        "shell_command_profile": "web",
    },
    "server": {
        "project_dir": "template_server",
        "config_path": "template_server/deploy_config.sh",
        "shell_dir": "deploy_server",
        "view": "💻Server",
        "parameter_profile": "server",
        "shell_command_profile": "server",
    },
    "nginx": {
        "project_dir": "template_nginx",
        "config_path": "template_nginx/deploy_config.sh",
        "shell_dir": "deploy_nginx",
        "view": "📻Nginx",
        "parameter_profile": "branch_only",
        "shell_command_profile": "nginx",
    },
    "ssl": {
        "project_dir": "remote/ssl",
        "config_path": "remote/ssl/deploy_config.sh",
        "shell_dir": "deploy_ssl",
        "view": "🔐SSL",
        "parameter_profile": "branch_only",
        "shell_command_profile": "ssl",
    },
}
SUPPORTED_TYPES = tuple(JOB_TYPE_META.keys())
GIT_PARAMETER_TAG = "net.uaznia.lukanus.hudson.plugins.gitparameter.GitParameterDefinition"
MANAGED_BY = "deploy_shell/deploy_jenkins"
MARKER_FILE_NAME = ".deploy_shell_managed_job.json"
PLACEHOLDER_PATTERN = re.compile(r"{{\s*([a-zA-Z0-9_]+)\s*}}")
CONFIG_PATH_PATTERN = re.compile(r'--config\s+"?\$WORKSPACE/([^"\n]+/deploy_config\.sh)"?')
LIST_VIEW_TAG = "listView"
ALL_VIEW_TAG = "hudson.model.AllView"
DEFAULT_VIEW_BY_TYPE = {job_type: meta["view"] for job_type, meta in JOB_TYPE_META.items()}
DEFAULT_MANAGED_VIEWS = ["📱App", "💻Server", "🧭Web", "📻Nginx", "🔐SSL"]
DEFAULT_LIST_VIEW_COLUMNS = [
    "hudson.views.StatusColumn",
    "hudson.views.WeatherColumn",
    "hudson.views.JobColumn",
    "hudson.views.LastSuccessColumn",
    "hudson.views.LastFailureColumn",
    "hudson.views.LastDurationColumn",
    "hudson.views.BuildButtonColumn",
]

SHELL_DIR_TO_TYPE = {meta["shell_dir"]: job_type for job_type, meta in JOB_TYPE_META.items()}


def fail(message: str) -> None:
    raise SystemExit(message)


def get_job_type_meta(job_type: str) -> dict:
    meta = JOB_TYPE_META.get(job_type)
    if not meta:
        fail(f"不支持的 job.type: {job_type}")
    return meta


def normalize_bool(value: object) -> bool:
    if isinstance(value, bool):
        return value
    if value is None:
        return False
    if isinstance(value, (int, float)):
        return bool(value)
    lowered = str(value).strip().lower()
    return lowered in {"1", "true", "yes", "y", "on"}


def make_node(
    tag: str,
    *,
    text: str | None = None,
    attributes: dict[str, str] | None = None,
    children: list[dict] | None = None,
) -> dict:
    node = {"tag": tag}
    if attributes:
        node["attributes"] = attributes
    if text is not None:
        node["text"] = text
    if children is not None:
        node["children"] = children
    return node


def build_branch_parameter_node(default_value: str, uuid_value: str) -> dict:
    return make_node(
        GIT_PARAMETER_TAG,
        attributes={"plugin": "git-parameter@460.v71e7583a_c099"},
        children=[
            make_node("name", text="BuildBranch"),
            make_node("description", text="构建分支"),
            make_node("uuid", text=uuid_value),
            make_node("type", text="PT_BRANCH"),
            make_node("branch", text=""),
            make_node("tagFilter", text="*"),
            make_node("branchFilter", text=".*"),
            make_node("sortMode", text="NONE"),
            make_node("defaultValue", text=default_value),
            make_node("selectedValue", text="NONE"),
            make_node("quickFilterEnabled", text="false"),
            make_node("listSize", text="5"),
            make_node("requiredParameter", text="false"),
        ],
    )


def build_choice_parameter_node(name: str, description: str, values: list[str]) -> dict:
    return make_node(
        "hudson.model.ChoiceParameterDefinition",
        children=[
            make_node("name", text=name),
            make_node("description", text=description),
            make_node(
                "choices",
                attributes={"class": "java.util.Arrays$ArrayList"},
                children=[
                    make_node(
                        "a",
                        attributes={"class": "string-array"},
                        children=[make_node("string", text=value) for value in values],
                    )
                ],
            ),
        ],
    )


def build_string_parameter_node(name: str, description: str, default_value: str) -> dict:
    return make_node(
        "hudson.model.StringParameterDefinition",
        children=[
            make_node("name", text=name),
            make_node("description", text=description),
            make_node("defaultValue", text=default_value),
            make_node("trim", text="true"),
        ],
    )


def default_parameter_profile_name(job_type: str) -> str:
    return get_job_type_meta(job_type)["parameter_profile"]


def default_view_name(job_type: str) -> str:
    return get_job_type_meta(job_type)["view"]


def default_project_dir(job_type: str) -> str:
    return get_job_type_meta(job_type)["project_dir"]


def default_config_path(job_type: str) -> str:
    return get_job_type_meta(job_type)["config_path"]


def default_remote_trigger_token(job_name: str) -> str:
    return f"{DEFAULT_REMOTE_TRIGGER_PREFIX}-{uuid.uuid5(uuid.NAMESPACE_URL, f'{MANAGED_BY}:{job_name}:remote-trigger')}"


def build_shared_shell_command_profiles() -> dict[str, str]:
    profiles = {}
    for job_type in ("swift_app", "android_app", "web", "server", "nginx", "ssl"):
        meta = get_job_type_meta(job_type)
        profiles[job_type] = (
            "set -euo pipefail\n\n"
            'cd "$WORKSPACE"\n'
            "git submodule sync --recursive\n"
            "git submodule update --init --remote --recursive deploy_shell\n\n"
            f'bash "$WORKSPACE/deploy_shell/{meta["shell_dir"]}/remote_deploy_pipeline.sh" '
            '--config "$WORKSPACE/{{config_path}}"'
        )
    return profiles


def build_shared_parameter_profiles() -> dict[str, dict]:
    branch_only = {
        "parameterized": True,
        "parameter_definitions": [
            build_branch_parameter_node("{{branch_parameter_default}}", "{{build_branch_uuid}}")
        ],
    }
    mobile_build_definitions = [
        build_branch_parameter_node("{{branch_parameter_default}}", "{{build_branch_uuid}}"),
        build_choice_parameter_node(
            "BuildType",
            "📦打包类型\napp-store（市场包）\nad-hoc    （测试包）",
            ["app-store", "ad-hoc"],
        ),
        build_choice_parameter_node(
            "BuildEnv",
            "🌍后端环境\ntest:  测试环境\nprod: 生产环境",
            ["test", "prod"],
        ),
        build_choice_parameter_node(
            "BuildPodUpdate",
            "🚀是否需要 pod update\nfalse:  ❌不需要\ntrue:   ✅需要",
            ["false", "true"],
        ),
    ]
    return {
        "branch_only": branch_only,
        "server": {
            "parameterized": True,
            "parameter_definitions": [
                build_branch_parameter_node("{{branch_parameter_default}}", "{{build_branch_uuid}}"),
                build_choice_parameter_node(
                    "BuildEnv",
                    "🌍后端环境\ntest:  测试环境\nprod: 生产环境",
                    ["test", "prod"],
                ),
            ],
        },
        "swift_app": {
            "parameterized": True,
            "parameter_definitions": mobile_build_definitions,
        },
        "android_app": {
            "parameterized": True,
            "parameter_definitions": [
                build_branch_parameter_node("{{branch_parameter_default}}", "{{build_branch_uuid}}"),
                build_choice_parameter_node(
                    "BuildEnv",
                    "🌍后端环境\ntest:  测试环境\nprod: 生产环境",
                    ["test", "prod"],
                ),
                build_string_parameter_node(
                    "BuildChannel",
                    "📺Android 渠道名；可填 official、huawei，或 all 表示全渠道",
                    "official",
                ),
                build_choice_parameter_node(
                    "BuildArtifact",
                    "📦Android 产物类型",
                    ["apk", "aab"],
                ),
            ],
        },
    }


def build_default_shared_profiles() -> dict:
    return {
        "parameter_profiles": build_shared_parameter_profiles(),
        "shell_command_profiles": build_shared_shell_command_profiles(),
    }


def parse_view_mapping_from_jenkins_config(config_path: str | None) -> dict[str, str]:
    if not config_path:
        return {}
    path = Path(config_path)
    if not path.exists():
        return {}

    try:
        root = ET.fromstring(path.read_text(encoding="utf-8"))
    except ET.ParseError:
        return {}

    mapping: dict[str, str] = {}
    for view in root.findall("./views/*"):
        if view.tag == ALL_VIEW_TAG:
            continue
        view_name = view.findtext("./name", default="").strip()
        if not view_name:
            continue
        for name_node in view.findall("./jobNames/string"):
            job_name = (name_node.text or "").strip()
            if job_name and job_name not in mapping:
                mapping[job_name] = view_name
    return mapping


def element_to_node(element: ET.Element) -> dict:
    children = [element_to_node(child) for child in list(element)]
    node = {"tag": element.tag}
    if element.attrib:
        node["attributes"] = dict(element.attrib)

    if children:
        if element.text and element.text.strip():
            node["text"] = element.text
        node["children"] = children
    else:
        node["text"] = element.text or ""
    return node


def node_to_element(node: dict) -> ET.Element:
    tag = node.get("tag")
    if not tag:
        fail("parameter_definitions 中存在缺少 tag 的节点")

    attributes = node.get("attributes") or {}
    if not isinstance(attributes, dict):
        fail(f"{tag}: attributes 必须是对象")

    element = ET.Element(str(tag), {str(key): str(value) for key, value in attributes.items()})
    text = node.get("text")
    if text is not None:
        element.text = str(text)

    children = node.get("children") or []
    if not isinstance(children, list):
        fail(f"{tag}: children 必须是数组")
    for child in children:
        if not isinstance(child, dict):
            fail(f"{tag}: children 中的元素必须是对象")
        element.append(node_to_element(child))
    return element


def default_parameter_definitions(job_name: str, job_type: str, branch_default: str) -> list[dict]:
    definitions = [
        build_branch_parameter_node(
            branch_default,
            str(uuid.uuid5(uuid.NAMESPACE_URL, f"{job_name}:BuildBranch")),
        )
    ]

    if job_type == "server":
        definitions.append(
            build_choice_parameter_node(
                "BuildEnv",
                "🌍后端环境\ntest:  测试环境\nprod: 生产环境",
                ["test", "prod"],
            )
        )
    elif job_type == "swift_app":
        definitions.extend(
            [
                build_choice_parameter_node(
                    "BuildType",
                    "📦打包类型\napp-store（市场包）\nad-hoc    （测试包）",
                    ["app-store", "ad-hoc"],
                ),
                build_choice_parameter_node(
                    "BuildEnv",
                    "🌍后端环境\ntest:  测试环境\nprod: 生产环境",
                    ["test", "prod"],
                ),
                build_choice_parameter_node(
                    "BuildPodUpdate",
                    "🚀是否需要 pod update\nfalse:  ❌不需要\ntrue:   ✅需要",
                    ["false", "true"],
                ),
            ]
        )
    elif job_type == "android_app":
        definitions.extend(
            [
                build_choice_parameter_node(
                    "BuildEnv",
                    "🌍后端环境\ntest:  测试环境\nprod: 生产环境",
                    ["test", "prod"],
                ),
                build_string_parameter_node(
                    "BuildChannel",
                    "📺Android 渠道名；可填 official、huawei，或 all 表示全渠道",
                    "official",
                ),
                build_choice_parameter_node(
                    "BuildArtifact",
                    "📦Android 产物类型",
                    ["apk", "aab"],
                ),
            ]
        )

    return definitions


def build_template_context(
    *,
    name: str,
    job_type: str,
    project_dir: str,
    config_path: str,
    branch_default: str,
) -> dict[str, str]:
    return {
        "name": name,
        "job_name": name,
        "type": job_type,
        "project_dir": project_dir,
        "config_path": config_path,
        "branch_parameter_default": branch_default,
        "build_branch_uuid": str(uuid.uuid5(uuid.NAMESPACE_URL, f"{name}:BuildBranch")),
    }


def render_template_string(template: str, context: dict[str, str]) -> str:
    def replacer(match: re.Match[str]) -> str:
        key = match.group(1)
        return context.get(key, match.group(0))

    return PLACEHOLDER_PATTERN.sub(replacer, template)


def render_node_template(node: dict, context: dict[str, str]) -> dict:
    rendered = {"tag": node["tag"]}
    attributes = node.get("attributes")
    if attributes:
        rendered["attributes"] = {
            key: render_template_string(str(value), context) for key, value in attributes.items()
        }
    if "text" in node:
        rendered["text"] = render_template_string(str(node.get("text", "")), context)
    if "children" in node:
        rendered["children"] = [render_node_template(child, context) for child in node.get("children", [])]
    return rendered


def normalize_parameter_nodes_for_profile_match(nodes: list[dict]) -> list[dict]:
    normalized = []
    for node in nodes:
        current = {"tag": node["tag"]}
        if node.get("attributes"):
            current["attributes"] = dict(node["attributes"])
        if "text" in node:
            current["text"] = "__dynamic__" if node["tag"] in {"uuid", "defaultValue", "description"} else node["text"]
        if "children" in node:
            current["children"] = normalize_parameter_nodes_for_profile_match(node["children"])
        normalized.append(current)
    return normalized


def suggest_shell_command_profile(job: dict) -> str | None:
    expected = build_shell_command(
        {
            "type": job["type"],
            "config_path": job["config_path"],
        }
    ).rstrip()
    return job["type"] if job["shell_command"].rstrip() == expected else None


def suggest_parameter_profile(job: dict, shared_profiles: dict) -> str | None:
    if not job["parameterized"]:
        return None

    actual = normalize_parameter_nodes_for_profile_match(job["parameter_definitions"])
    context = build_template_context(
        name=job["name"],
        job_type=job["type"],
        project_dir=job["project_dir"],
        config_path=job["config_path"],
        branch_default=job["branch_parameter_default"],
    )
    for profile_name, profile in shared_profiles["parameter_profiles"].items():
        if not normalize_bool(profile.get("parameterized", True)):
            continue
        rendered = [
            render_node_template(node, context)
            for node in profile.get("parameter_definitions") or []
        ]
        if normalize_parameter_nodes_for_profile_match(rendered) == actual:
            return profile_name
    return None


def compact_job_for_config(job: dict, shared_profiles: dict) -> dict:
    compact = {
        "name": job["name"],
        "type": job["type"],
        "repo_url": job["repo_url"],
    }
    if job["description"]:
        compact["description"] = job["description"]
    if job.get("view"):
        compact["view"] = job["view"]
    if job["branch_parameter_default"] != DEFAULT_BRANCH_PARAMETER:
        compact["branch_parameter_default"] = job["branch_parameter_default"]
    if job["disabled"]:
        compact["disabled"] = True
    if job["project_dir"] != default_project_dir(job["type"]):
        compact["project_dir"] = job["project_dir"]
    if job["config_path"] != default_config_path(job["type"]):
        compact["config_path"] = job["config_path"]
    if not job["remote_trigger_enabled"]:
        compact["remote_trigger_enabled"] = False
    elif job["remote_trigger_token"] != default_remote_trigger_token(job["name"]):
        compact["remote_trigger_token"] = job["remote_trigger_token"]

    parameter_profile = suggest_parameter_profile(job, shared_profiles)
    if parameter_profile is not None:
        compact["parameter_profile"] = parameter_profile
    else:
        compact["parameterized"] = job["parameterized"]
        compact["parameter_definitions"] = job["parameter_definitions"]

    shell_command_profile = suggest_shell_command_profile(job)
    if shell_command_profile is not None:
        compact["shell_command_profile"] = shell_command_profile
    else:
        compact["shell_command"] = job["shell_command"]

    return compact


def load_json_object(path: Path, label: str) -> dict:
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        fail(f"{label} 不存在: {path}")
    except json.JSONDecodeError as exc:
        fail(f"{label} 不是合法 JSON: {path} ({exc})")
    if not isinstance(payload, dict):
        fail(f"{label} 顶层必须是对象: {path}")
    return payload


def default_shared_config_filename(config_path: Path) -> str:
    if config_path.name == "jobs_config.json":
        return "jobs_shared.json"
    suffix = config_path.suffix or ".json"
    return f"{config_path.stem}_shared{suffix}"


def resolve_shared_config_reference(config_path: Path, payload: dict | None = None) -> str:
    if payload:
        shared_file = str(payload.get("shared_file") or "").strip()
        if shared_file:
            return shared_file
    return default_shared_config_filename(config_path)


def resolve_shared_config_path(config_path: Path, payload: dict | None = None) -> tuple[str, Path]:
    reference = resolve_shared_config_reference(config_path, payload)
    shared_path = Path(reference).expanduser()
    if not shared_path.is_absolute():
        shared_path = (config_path.parent / shared_path).resolve()
    return reference, shared_path


def load_payload_with_shared(config_path: str) -> dict:
    path = Path(config_path).expanduser().resolve()
    payload = load_json_object(path, "配置文件")
    if payload.get("version") != DEFAULT_CONFIG_VERSION:
        fail(f"配置文件 version 必须是 {DEFAULT_CONFIG_VERSION}")

    inline_shared = payload.get("shared")
    if inline_shared is not None:
        shared_payload = inline_shared
    else:
        shared_file = str(payload.get("shared_file") or "").strip()
        if shared_file:
            _, shared_path = resolve_shared_config_path(path, payload)
            if not shared_path.is_file():
                fail(f"shared_file 指向的文件不存在: {shared_path}")
            shared_container = load_json_object(shared_path, "共享配置文件")
            shared_payload = shared_container.get("shared", shared_container)
        else:
            auto_shared_path = (path.parent / default_shared_config_filename(path)).resolve()
            if auto_shared_path.is_file():
                shared_container = load_json_object(auto_shared_path, "共享配置文件")
                shared_payload = shared_container.get("shared", shared_container)
            else:
                shared_payload = build_default_shared_profiles()

    merged = dict(payload)
    merged["shared"] = shared_payload
    return merged


def load_shared_profiles(config_path: str | None) -> dict:
    if not config_path:
        return build_default_shared_profiles()
    path = Path(config_path).expanduser()
    if not path.exists():
        return build_default_shared_profiles()
    return normalize_shared_profiles(load_payload_with_shared(str(path)))


def load_shared_config_reference(config_path: str | None) -> str | None:
    if not config_path:
        return None
    path = Path(config_path).expanduser()
    if not path.exists():
        return None
    payload = load_json_object(path, "配置文件")
    if payload.get("version") != DEFAULT_CONFIG_VERSION:
        fail(f"配置文件 version 必须是 {DEFAULT_CONFIG_VERSION}")
    reference = str(payload.get("shared_file") or "").strip()
    return reference or None


def write_json_object(path: Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def write_config_payload(config_path: str, payload: dict) -> None:
    output_path = Path(config_path).expanduser().resolve()
    shared_profiles = normalize_shared_profiles({"shared": payload.get("shared") or build_default_shared_profiles()})
    main_payload = dict(payload)
    main_payload.pop("shared", None)

    shared_reference, shared_path = resolve_shared_config_path(output_path, main_payload)
    ordered_payload: dict[str, object] = {}
    if "version" in main_payload:
        ordered_payload["version"] = main_payload.pop("version")
    ordered_payload["shared_file"] = shared_reference
    for key in ("jobs", "commented_jobs"):
        if key in main_payload:
            ordered_payload[key] = main_payload.pop(key)
    ordered_payload.update(main_payload)

    write_json_object(shared_path, shared_profiles)
    write_json_object(output_path, ordered_payload)


def normalize_shared_profiles(payload: dict) -> dict:
    shared = payload.get("shared") or {}
    if not isinstance(shared, dict):
        fail("shared 必须是对象")

    parameter_profiles = shared.get("parameter_profiles") or {}
    shell_command_profiles = shared.get("shell_command_profiles") or {}
    if not isinstance(parameter_profiles, dict):
        fail("shared.parameter_profiles 必须是对象")
    if not isinstance(shell_command_profiles, dict):
        fail("shared.shell_command_profiles 必须是对象")
    for profile_name, shell_command in shell_command_profiles.items():
        if not isinstance(shell_command, str):
            fail(f"shared.shell_command_profiles.{profile_name} 必须是字符串")
    for profile_name, profile in parameter_profiles.items():
        if not isinstance(profile, dict):
            fail(f"shared.parameter_profiles.{profile_name} 必须是对象")
        definitions = profile.get("parameter_definitions") or []
        if not isinstance(definitions, list):
            fail(f"shared.parameter_profiles.{profile_name}.parameter_definitions 必须是数组")
        for node in definitions:
            if not isinstance(node, dict):
                fail(f"shared.parameter_profiles.{profile_name}.parameter_definitions 中的元素必须是对象")
            node_to_element(node)

    return {
        "parameter_profiles": parameter_profiles,
        "shell_command_profiles": shell_command_profiles,
    }


def normalize_job(job: dict, shared_profiles: dict | None = None) -> dict:
    shared_profiles = shared_profiles or {"parameter_profiles": {}, "shell_command_profiles": {}}

    name = str(job.get("name", "")).strip()
    raw_job_type = str(job.get("type", "")).strip()
    repo_url = str(job.get("repo_url", "")).strip()
    description = str(job.get("description", "")).strip()
    view = str(job.get("view") or "").strip()
    shell_command = str(job.get("shell_command") or "").rstrip()
    config_path = str(job.get("config_path") or "").strip()
    if not config_path and shell_command:
        config_match = CONFIG_PATH_PATTERN.search(shell_command)
        if config_match:
            config_path = config_match.group(1).strip()
    project_dir = str(job.get("project_dir") or "").strip()
    if not project_dir and config_path:
        project_dir = PurePosixPath(config_path).parent.as_posix()
    branch_default = str(job.get("branch_parameter_default") or DEFAULT_BRANCH_PARAMETER).strip()
    disabled = normalize_bool(job.get("disabled", False))
    remote_trigger_enabled_raw = job.get("remote_trigger_enabled")
    remote_trigger_enabled = normalize_bool(
        remote_trigger_enabled_raw if remote_trigger_enabled_raw is not None else True
    )
    remote_trigger_token = str(job.get("remote_trigger_token") or "").strip()
    parameterized_raw = job.get("parameterized")
    parameterized = normalize_bool(parameterized_raw if parameterized_raw is not None else True)
    parameter_definitions = job.get("parameter_definitions")

    if not name:
        fail("job.name 不能为空")
    if raw_job_type not in SUPPORTED_TYPES:
        fail(f"job.type 仅支持 {', '.join(SUPPORTED_TYPES)}，当前: {raw_job_type or '<empty>'}")
    job_type = raw_job_type
    if not project_dir:
        project_dir = default_project_dir(job_type)
    if not config_path:
        config_path = default_config_path(job_type)
    parameter_profile = str(job.get("parameter_profile") or default_parameter_profile_name(job_type)).strip()
    shell_command_profile = str(job.get("shell_command_profile") or get_job_type_meta(job_type)["shell_command_profile"]).strip()
    project_dir = PurePosixPath(project_dir).as_posix()
    config_path = PurePosixPath(config_path).as_posix()
    if name in {".", ".."} or "/" in name:
        fail(f"job.name 不能包含路径分隔符，当前: {name}")
    if not repo_url:
        fail(f"{name}: repo_url 不能为空")
    if not project_dir:
        fail(f"{name}: project_dir 不能为空")
    if not config_path:
        fail(f"{name}: config_path 不能为空")
    if config_path.startswith("/"):
        fail(f"{name}: config_path 必须是相对 WORKSPACE 的相对路径，当前: {config_path}")
    if project_dir.startswith("/"):
        fail(f"{name}: project_dir 必须是相对 WORKSPACE 的相对路径，当前: {project_dir}")
    if PurePosixPath(config_path).name != "deploy_config.sh":
        fail(f"{name}: config_path 必须指向 deploy_config.sh，当前: {config_path}")
    config_project_dir = PurePosixPath(config_path).parent.as_posix()
    if config_project_dir in {"", "."}:
        fail(f"{name}: config_path 必须位于某个项目目录下，当前: {config_path}")
    if project_dir != config_project_dir:
        fail(f"{name}: project_dir 必须与 config_path 的父目录一致，当前: {project_dir} != {config_project_dir}")
    if remote_trigger_enabled and not remote_trigger_token:
        remote_trigger_token = default_remote_trigger_token(name)
    if not remote_trigger_enabled:
        remote_trigger_token = ""
    if remote_trigger_token and re.search(r"\s", remote_trigger_token):
        fail(f"{name}: remote_trigger_token 不能包含空白字符")

    context = build_template_context(
        name=name,
        job_type=job_type,
        project_dir=project_dir,
        config_path=config_path,
        branch_default=branch_default,
    )
    if not shell_command:
        shell_template = shared_profiles["shell_command_profiles"].get(shell_command_profile)
        if shell_template is not None:
            shell_command = render_template_string(str(shell_template), context).rstrip()
        else:
            shell_command = build_shell_command(
                {
                    "type": job_type,
                    "config_path": config_path,
                }
            )
    if parameter_definitions is None:
        profile = shared_profiles["parameter_profiles"].get(parameter_profile)
        if profile is not None:
            if not isinstance(profile, dict):
                fail(f"{name}: shared.parameter_profiles.{parameter_profile} 必须是对象")
            profile_parameterized = normalize_bool(profile.get("parameterized", True))
            profile_definitions = profile.get("parameter_definitions") or []
            if not isinstance(profile_definitions, list):
                fail(f"{name}: shared.parameter_profiles.{parameter_profile}.parameter_definitions 必须是数组")
            parameterized = profile_parameterized if parameterized_raw is None else parameterized
            parameter_definitions = [render_node_template(node, context) for node in profile_definitions]
        else:
            parameter_definitions = default_parameter_definitions(name, job_type, branch_default)
    if not isinstance(parameter_definitions, list):
        fail(f"{name}: parameter_definitions 必须是数组")
    if not parameterized and parameter_definitions:
        fail(f"{name}: parameterized=false 时 parameter_definitions 必须为空")
    for node in parameter_definitions:
        if not isinstance(node, dict):
            fail(f"{name}: parameter_definitions 中的元素必须是对象")
        node_to_element(node)

    return {
        "name": name,
        "type": job_type,
        "repo_url": repo_url,
        "description": description,
        "view": view,
        "project_dir": project_dir,
        "config_path": config_path,
        "branch_parameter_default": branch_default or DEFAULT_BRANCH_PARAMETER,
        "disabled": disabled,
        "remote_trigger_enabled": remote_trigger_enabled,
        "remote_trigger_token": remote_trigger_token,
        "parameter_profile": parameter_profile,
        "shell_command_profile": shell_command_profile,
        "shell_command": shell_command,
        "parameterized": parameterized,
        "parameter_definitions": parameter_definitions,
    }


def indent_tree(element: ET.Element, level: int = 0) -> None:
    indent = "\n" + level * "  "
    if len(element):
        if not element.text or not element.text.strip():
            element.text = indent + "  "
        for child in element:
            indent_tree(child, level + 1)
        if not element[-1].tail or not element[-1].tail.strip():
            element[-1].tail = indent
    elif level and (not element.tail or not element.tail.strip()):
        element.tail = indent


def add_text_element(parent: ET.Element, tag: str, text: str) -> ET.Element:
    element = ET.SubElement(parent, tag)
    element.text = text
    return element


def build_shell_command(job: dict) -> str:
    job_type = job["type"]
    config_path = job["config_path"]
    shell_dir = get_job_type_meta(job_type)["shell_dir"]
    return (
        "set -euo pipefail\n\n"
        'cd "$WORKSPACE"\n'
        "git submodule sync --recursive\n"
        "git submodule update --init --remote --recursive deploy_shell\n\n"
        f'bash "$WORKSPACE/deploy_shell/{shell_dir}/remote_deploy_pipeline.sh" '
        f'--config "$WORKSPACE/{config_path}"'
    )


def build_job_xml(job: dict) -> str:
    project = ET.Element("project")
    ET.SubElement(project, "actions")
    add_text_element(project, "description", job["description"])
    add_text_element(project, "keepDependencies", "false")
    if job["remote_trigger_enabled"] and job["remote_trigger_token"]:
        add_text_element(project, "authToken", job["remote_trigger_token"])

    properties = ET.SubElement(project, "properties")
    if job["parameterized"]:
        params_property = ET.SubElement(properties, "hudson.model.ParametersDefinitionProperty")
        parameter_definitions = ET.SubElement(params_property, "parameterDefinitions")
        for node in job["parameter_definitions"]:
            parameter_definitions.append(node_to_element(node))

    scm = ET.SubElement(project, "scm", {"class": "hudson.plugins.git.GitSCM"})
    add_text_element(scm, "configVersion", "2")
    user_remote_configs = ET.SubElement(scm, "userRemoteConfigs")
    remote_config = ET.SubElement(user_remote_configs, "hudson.plugins.git.UserRemoteConfig")
    add_text_element(remote_config, "url", job["repo_url"])

    branches = ET.SubElement(scm, "branches")
    branch_spec = ET.SubElement(branches, "hudson.plugins.git.BranchSpec")
    add_text_element(branch_spec, "name", "$BuildBranch")
    add_text_element(scm, "doGenerateSubmoduleConfigurations", "false")
    ET.SubElement(scm, "submoduleCfg", {"class": "empty-list"})
    ET.SubElement(scm, "extensions")

    add_text_element(project, "canRoam", "true")
    add_text_element(project, "disabled", "true" if job["disabled"] else "false")
    add_text_element(project, "blockBuildWhenDownstreamBuilding", "false")
    add_text_element(project, "blockBuildWhenUpstreamBuilding", "false")
    ET.SubElement(project, "triggers")
    add_text_element(project, "concurrentBuild", "false")

    builders = ET.SubElement(project, "builders")
    shell = ET.SubElement(builders, "hudson.tasks.Shell")
    add_text_element(shell, "command", job["shell_command"])
    ET.SubElement(shell, "configuredLocalRules")

    ET.SubElement(project, "publishers")
    ET.SubElement(project, "buildWrappers")

    indent_func = getattr(ET, "indent", None)
    if callable(indent_func):
        indent_func(project, space="  ")
    else:
        indent_tree(project)
    body = ET.tostring(project, encoding="unicode")
    return "<?xml version='1.1' encoding='UTF-8'?>\n" + body + "\n"


def parse_job_from_xml(job_name: str, xml_text: str) -> dict | None:
    try:
        root = ET.fromstring(xml_text)
    except ET.ParseError:
        return None

    command = root.findtext("./builders/hudson.tasks.Shell/command", default="")
    type_match = re.search(r"deploy_shell/(deploy_[a-z_]+)/remote_deploy_pipeline\.sh", command)
    if not type_match:
        return None
    config_match = CONFIG_PATH_PATTERN.search(command)
    if not config_match:
        return None

    shell_dir = type_match.group(1)
    job_type = SHELL_DIR_TO_TYPE.get(shell_dir)
    if not job_type:
        return None
    config_path = config_match.group(1)
    repo_url = root.findtext("./scm/userRemoteConfigs/hudson.plugins.git.UserRemoteConfig/url", default="").strip()
    if not repo_url:
        return None
    project_dir = PurePosixPath(config_path).parent.as_posix()
    if project_dir in {"", "."}:
        print(f"skip remote job {job_name}: 无法从 config_path 推导 project_dir: {config_path}", file=sys.stderr)
        return None

    description = root.findtext("./description", default="")
    branch_default = root.findtext(f".//{GIT_PARAMETER_TAG}/defaultValue", default=DEFAULT_BRANCH_PARAMETER)
    disabled = root.findtext("./disabled", default="false").strip().lower() == "true"
    remote_trigger_token = root.findtext("./authToken", default="").strip()
    parameter_definitions_element = root.find("./properties/hudson.model.ParametersDefinitionProperty/parameterDefinitions")
    parameterized = parameter_definitions_element is not None
    parameter_definitions = []
    if parameter_definitions_element is not None:
        parameter_definitions = [element_to_node(child) for child in list(parameter_definitions_element)]

    return {
        "name": job_name,
        "type": job_type,
        "repo_url": repo_url,
        "description": description,
        "project_dir": project_dir,
        "config_path": config_path,
        "branch_parameter_default": branch_default or DEFAULT_BRANCH_PARAMETER,
        "disabled": disabled,
        "remote_trigger_enabled": bool(remote_trigger_token),
        "remote_trigger_token": remote_trigger_token,
        "shell_command": command,
        "parameterized": parameterized,
        "parameter_definitions": parameter_definitions,
    }


def command_import_remote(args: argparse.Namespace) -> int:
    shared_profiles = load_shared_profiles(args.output)
    view_mapping = parse_view_mapping_from_jenkins_config(args.jenkins_config)
    jobs = []
    for raw_line in Path(args.input).read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line:
            continue
        try:
            job_name, encoded = line.split("\t", 1)
        except ValueError:
            continue
        try:
            xml_text = base64.b64decode(encoded).decode("utf-8")
        except Exception:
            continue
        job = parse_job_from_xml(job_name, xml_text)
        if job is not None:
            if view_mapping.get(job_name):
                job["view"] = view_mapping[job_name]
            jobs.append(job)

    jobs.sort(key=lambda item: item["name"])
    output_path = Path(args.output)
    payload = {
        "version": DEFAULT_CONFIG_VERSION,
        "shared": shared_profiles,
        "jobs": [compact_job_for_config(job, shared_profiles) for job in jobs],
    }
    shared_reference = load_shared_config_reference(str(output_path))
    if shared_reference:
        payload["shared_file"] = shared_reference
    _, existing_commented_jobs, _ = load_commented_jobs(str(output_path))
    if existing_commented_jobs:
        payload["commented_jobs"] = [
            compact_job_for_config(job, shared_profiles)
            for job in existing_commented_jobs
        ]
    write_config_payload(str(output_path), payload)
    return 0


def load_commented_jobs(config_path: str | None) -> tuple[set[str], list[dict], dict[str, dict]]:
    if not config_path:
        return set(), [], {}
    path = Path(config_path)
    if not path.exists():
        return set(), [], {}

    payload = load_payload_with_shared(str(path))
    commented_jobs = payload.get("commented_jobs") or []
    if not isinstance(commented_jobs, list):
        fail("commented_jobs 必须是数组")

    shared_profiles = normalize_shared_profiles(payload)
    normalized = []
    for item in commented_jobs:
        if not isinstance(item, dict):
            fail("commented_jobs 中的元素必须是对象")
        normalized.append(normalize_job(item, shared_profiles))
    by_name = {job["name"]: job for job in normalized}
    return set(by_name), normalized, by_name


def load_config(config_path: str) -> list[dict]:
    payload = load_payload_with_shared(config_path)
    shared_profiles = normalize_shared_profiles(payload)
    jobs = payload.get("jobs")
    if not isinstance(jobs, list):
        fail("配置文件必须包含 jobs 数组")

    normalized = [normalize_job(item, shared_profiles) for item in jobs]
    names = [job["name"] for job in normalized]
    if len(names) != len(set(names)):
        fail("配置文件中的 job.name 不能重复")
    normalized.sort(key=lambda item: item["name"])
    return normalized


def command_render(args: argparse.Namespace) -> int:
    jobs = load_config(args.config)
    output_dir = Path(args.out_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    manifest = {"managed_by": MANAGED_BY, "marker_file": MARKER_FILE_NAME, "jobs": []}
    for job in jobs:
        job_dir = output_dir / job["name"]
        job_dir.mkdir(parents=True, exist_ok=True)
        (job_dir / "config.xml").write_text(build_job_xml(job), encoding="utf-8")
        marker_payload = {
            "managed_by": MANAGED_BY,
            "name": job["name"],
            "type": job["type"],
            "repo_url": job["repo_url"],
            "config_path": job["config_path"],
            "remote_trigger_enabled": job["remote_trigger_enabled"],
            "remote_trigger_token": job["remote_trigger_token"],
        }
        (job_dir / MARKER_FILE_NAME).write_text(
            json.dumps(marker_payload, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
        manifest["jobs"].append(job)

    (output_dir / "manifest.json").write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
    return 0


def command_list_http_triggers(args: argparse.Namespace) -> int:
    jobs = load_config(args.config)
    job_type_filter = (args.type or "").strip()
    if job_type_filter:
        if job_type_filter not in SUPPORTED_TYPES:
            fail(f"--type 仅支持 {', '.join(SUPPORTED_TYPES)}，当前: {job_type_filter}")
        jobs = [job for job in jobs if job["type"] == job_type_filter]

    payload = []
    for job in jobs:
        if not job["remote_trigger_enabled"] or not job["remote_trigger_token"]:
            continue
        payload.append(
            {
                "name": job["name"],
                "type": job["type"],
                "branch_parameter_default": job["branch_parameter_default"],
                "parameterized": job["parameterized"],
                "remote_trigger_token": job["remote_trigger_token"],
            }
        )

    print(json.dumps(payload, ensure_ascii=False, indent=2))
    return 0


def set_list_view_job_names(view: ET.Element, job_names: list[str]) -> None:
    job_names_element = view.find("./jobNames")
    if job_names_element is None:
        job_names_element = ET.SubElement(view, "jobNames")
    else:
        job_names_element.clear()
    ET.SubElement(job_names_element, "comparator", {"class": "java.lang.String$CaseInsensitiveComparator"})
    for job_name in sorted(set(job_names), key=str.lower):
        add_text_element(job_names_element, "string", job_name)


def ensure_list_view_defaults(view: ET.Element) -> None:
    owner = view.find("./owner")
    if owner is None:
        owner = ET.Element("owner", {"class": "hudson"})
        owner.set("reference", "../../..")
        view.insert(0, owner)

    defaults = {
        "filterExecutors": "false",
        "filterQueue": "false",
        "recurse": "false",
    }
    for tag, text in defaults.items():
        element = view.find(f"./{tag}")
        if element is None:
            add_text_element(view, tag, text)
        elif not (element.text or "").strip():
            element.text = text

    properties = view.find("./properties")
    if properties is None:
        properties = ET.SubElement(view, "properties")
    if "class" not in properties.attrib:
        properties.set("class", "hudson.model.View$PropertyList")

    if view.find("./jobFilters") is None:
        ET.SubElement(view, "jobFilters")

    columns = view.find("./columns")
    if columns is None:
        columns = ET.SubElement(view, "columns")
        for tag in DEFAULT_LIST_VIEW_COLUMNS:
            ET.SubElement(columns, tag)


def build_list_view(name: str, template: ET.Element | None = None) -> ET.Element:
    view = copy.deepcopy(template) if template is not None else ET.Element(LIST_VIEW_TAG)
    view.tag = LIST_VIEW_TAG
    name_element = view.find("./name")
    if name_element is None:
        name_element = ET.Element("name")
        insert_at = 1 if view.find("./owner") is not None else 0
        view.insert(insert_at, name_element)
    name_element.text = name
    ensure_list_view_defaults(view)
    return view


def command_rewrite_views(args: argparse.Namespace) -> int:
    manifest_path = Path(args.manifest)
    payload = json.loads(manifest_path.read_text(encoding="utf-8"))
    jobs = payload.get("jobs") or []
    if not isinstance(jobs, list):
        fail("manifest.jobs 必须是数组")

    root_path = Path(args.jenkins_config)
    root = ET.fromstring(root_path.read_text(encoding="utf-8"))
    views_container = root.find("./views")
    if views_container is None:
        views_container = ET.SubElement(root, "views")

    existing_views: dict[str, ET.Element] = {}
    template_view: ET.Element | None = None
    for view in views_container.findall("./*"):
        if view.tag == LIST_VIEW_TAG and template_view is None:
            template_view = view
        if view.tag == ALL_VIEW_TAG:
            continue
        view_name = view.findtext("./name", default="").strip()
        if view_name:
            existing_views[view_name] = view

    desired_views: dict[str, list[str]] = {}
    for item in jobs:
        if not isinstance(item, dict):
            continue
        job_name = str(item.get("name") or "").strip()
        view_name = str(item.get("view") or "").strip()
        if job_name and view_name:
            desired_views.setdefault(view_name, []).append(job_name)

    managed_view_names = list(dict.fromkeys(DEFAULT_MANAGED_VIEWS + sorted(desired_views)))
    for view_name in managed_view_names:
        job_names = desired_views.get(view_name, [])
        current = existing_views.get(view_name)
        if current is None:
            current = build_list_view(view_name, template_view)
            views_container.append(current)
            existing_views[view_name] = current
        set_list_view_job_names(current, job_names)

    indent_func = getattr(ET, "indent", None)
    if callable(indent_func):
        indent_func(root, space="  ")
    else:
        indent_tree(root)
    body = ET.tostring(root, encoding="unicode")
    root_path.write_text("<?xml version='1.1' encoding='UTF-8'?>\n" + body + "\n", encoding="utf-8")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="deploy_shell Jenkins job 同步辅助脚本")
    subparsers = parser.add_subparsers(dest="command", required=True)

    import_remote = subparsers.add_parser("import-remote", help="从远端导出的 config.xml 数据生成本地 jobs_config.json")
    import_remote.add_argument("--input", required=True, help="由 sync_jobs.sh pull 生成的临时文件")
    import_remote.add_argument("--output", required=True, help="输出配置文件路径")
    import_remote.add_argument("--jenkins-config", help="Jenkins 根 config.xml 路径，用于解析视图")
    import_remote.set_defaults(func=command_import_remote)

    render = subparsers.add_parser("render", help="根据 jobs_config.json 渲染 Jenkins job config.xml")
    render.add_argument("--config", required=True, help="jobs_config.json 路径")
    render.add_argument("--out-dir", required=True, help="渲染输出目录")
    render.set_defaults(func=command_render)

    list_http = subparsers.add_parser("list-http-triggers", help="输出 HTTP 触发清单")
    list_http.add_argument("--config", required=True, help="jobs_config.json 路径")
    list_http.add_argument("--type", help="按 job.type 过滤")
    list_http.set_defaults(func=command_list_http_triggers)

    rewrite_views = subparsers.add_parser("rewrite-views", help="根据 manifest.json 更新 Jenkins 根视图配置")
    rewrite_views.add_argument("--manifest", required=True, help="render 生成的 manifest.json 路径")
    rewrite_views.add_argument("--jenkins-config", required=True, help="Jenkins 根 config.xml 路径")
    rewrite_views.set_defaults(func=command_rewrite_views)
    return parser


def main(argv: Iterable[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(list(argv) if argv is not None else None)
    return args.func(args)


if __name__ == "__main__":
    sys.exit(main())
