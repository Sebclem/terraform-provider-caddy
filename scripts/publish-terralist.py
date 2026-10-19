#!/usr/bin/env python3
"""Publish a built Terraform provider release from Gitea to Terralist."""

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request


def normalize_version(version: str) -> str:
    """Strip leading 'v' and surrounding whitespace from a version string."""
    v = version.strip()
    if v.startswith("v"):
        v = v[1:]
    return v


def parse_platforms(checksum_text: str, version: str, host: str, owner: str, repo: str) -> list[dict]:
    """Parse SHA256SUMS text and extract platform entries for zip archives."""
    clean_version = normalize_version(version)
    platforms = []
    # Pattern matching: <checksum>  terraform-provider-<name>_<version>_<os>_<arch>.zip
    pattern = re.compile(r"^([a-f0-9]{64})\s+([^\s]+)$")

    for line in checksum_text.splitlines():
        line = line.strip()
        if not line:
            continue
        match = pattern.match(line)
        if not match:
            continue
        shasum, filename = match.group(1), match.group(2)
        if not filename.endswith(".zip"):
            continue

        # Extract os and arch from filename
        # Expected: terraform-provider-caddy_<version>_<os>_<arch>.zip
        parts = filename[:-4].split("_")
        if len(parts) >= 4:
            os_name = parts[-2]
            arch_name = parts[-1]
            download_url = f"https://{host}/{owner}/{repo}/releases/download/v{clean_version}/{filename}"
            platforms.append({
                "os": os_name,
                "arch": arch_name,
                "download_url": download_url,
                "filename": filename,
                "shasum": shasum,
            })

    return platforms


def build_payload(checksum_text: str, version: str, host: str, owner: str, repo: str) -> dict:
    """Build the Terralist provider version upload JSON payload."""
    clean_version = normalize_version(version)
    shasums_filename = f"terraform-provider-caddy_{clean_version}_SHA256SUMS"
    sig_filename = f"{shasums_filename}.sig"

    shasums_url = f"https://{host}/{owner}/{repo}/releases/download/v{clean_version}/{shasums_filename}"
    sig_url = f"https://{host}/{owner}/{repo}/releases/download/v{clean_version}/{sig_filename}"

    platforms = parse_platforms(checksum_text, clean_version, host, owner, repo)

    return {
        "protocols": ["6.0"],
        "shasums": {
            "url": shasums_url,
            "signature_url": sig_url,
        },
        "platforms": platforms,
    }


def publish_to_terralist(
    terralist_url: str,
    api_key: str,
    namespace: str,
    provider_name: str,
    version: str,
    payload: dict,
) -> None:
    """Send upload request to Terralist API endpoint."""
    clean_version = normalize_version(version)
    endpoint = f"{terralist_url.rstrip('/')}/v1/api/providers/{namespace}/{provider_name}/{clean_version}/upload"

    data = json.dumps(payload, indent=2).encode("utf-8")
    req = urllib.request.Request(
        endpoint,
        data=data,
        method="POST",
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer x-api-key:{api_key}",
            "X-API-Key": api_key,
        },
    )

    try:
        with urllib.request.urlopen(req) as resp:
            status = resp.getcode()
            body = resp.read().decode("utf-8")
            print(f"Terralist responded with HTTP {status}: {body}")
    except urllib.error.HTTPError as e:
        error_body = e.read().decode("utf-8") if e.fp else ""
        raise RuntimeError(
            f"Failed to publish to Terralist ({endpoint}): HTTP {e.code} {e.reason}\nResponse: {error_body}"
        ) from e
    except urllib.error.URLError as e:
        raise RuntimeError(f"Network error connecting to Terralist ({endpoint}): {e.reason}") from e


def main() -> int:
    parser = argparse.ArgumentParser(description="Publish provider release to Terralist")
    parser.add_argument("--version", default=os.environ.get("VERSION"), help="Provider version tag (e.g. v0.2.0)")
    parser.add_argument("--terralist-url", default=os.environ.get("TERRALIST_URL", "https://terralist.sebclem.fr"), help="Terralist base URL")
    parser.add_argument("--api-key", default=os.environ.get("TERRALIST_API_KEY"), help="Terralist API key")
    parser.add_argument("--dist-dir", default=os.environ.get("DIST_DIR", "dist"), help="Path to dist directory containing checksums and signatures")
    parser.add_argument("--host", default=os.environ.get("GITEA_HOST", "git.sebclem.fr"), help="Gitea hostname")
    parser.add_argument("--owner", default=os.environ.get("REPO_OWNER", "sebclem"), help="Repository owner")
    parser.add_argument("--repo", default=os.environ.get("REPO_NAME", "terraform-provider-caddy"), help="Repository name")
    parser.add_argument("--namespace", default=os.environ.get("TERRALIST_NAMESPACE", "sebclem"), help="Terralist authority/namespace")
    parser.add_argument("--provider-name", default=os.environ.get("TERRALIST_PROVIDER", "caddy"), help="Terralist provider name")

    args = parser.parse_args()

    if not args.version:
        print("Error: --version or VERSION env var is required.", file=sys.stderr)
        return 1

    if not args.api_key:
        print("Error: --api-key or TERRALIST_API_KEY env var is required.", file=sys.stderr)
        return 1

    clean_version = normalize_version(args.version)
    sums_filename = f"terraform-provider-caddy_{clean_version}_SHA256SUMS"
    sig_filename = f"{sums_filename}.sig"

    sums_path = os.path.join(args.dist_dir, sums_filename)
    sig_path = os.path.join(args.dist_dir, sig_filename)

    if not os.path.isfile(sums_path):
        print(f"Error: Checksums file not found: {sums_path}", file=sys.stderr)
        return 1

    if not os.path.isfile(sig_path):
        print(f"Error: Signature file not found: {sig_path}", file=sys.stderr)
        return 1

    with open(sums_path, "r", encoding="utf-8") as f:
        checksum_text = f.read()

    payload = build_payload(checksum_text, clean_version, args.host, args.owner, args.repo)
    if not payload["platforms"]:
        print(f"Error: No platform archives found in {sums_path}", file=sys.stderr)
        return 1

    print(f"Publishing {args.namespace}/{args.provider_name} v{clean_version} ({len(payload['platforms'])} platforms) to {args.terralist_url}...")
    try:
        publish_to_terralist(
            terralist_url=args.terralist_url,
            api_key=args.api_key,
            namespace=args.namespace,
            provider_name=args.provider_name,
            version=clean_version,
            payload=payload,
        )
    except Exception as e:
        print(f"Error publishing to Terralist: {e}", file=sys.stderr)
        return 1

    print("Successfully published to Terralist.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
