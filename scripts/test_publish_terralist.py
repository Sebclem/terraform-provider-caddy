import importlib.util
import os
import sys
import unittest
from unittest.mock import patch, MagicMock

# Load publish-terralist.py which contains a hyphen
script_path = os.path.join(os.path.dirname(__file__), "publish-terralist.py")
spec = importlib.util.spec_from_file_location("publish_terralist", script_path)
publish_terralist = importlib.util.module_from_spec(spec)
sys.modules["publish_terralist"] = publish_terralist
spec.loader.exec_module(publish_terralist)

class TestPublishTerralist(unittest.TestCase):
    def setUp(self):
        self.sample_sums = (
            "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2  terraform-provider-caddy_0.2.0_darwin_amd64.zip\n"
            "b5d4045c3f466fa91fe2cc6abe79232a1a57cdf104f7cf4e730109d43433eabb  terraform-provider-caddy_0.2.0_linux_amd64.zip\n"
            "a1b2c3d4e5f60718293a4b5c6d7e8f901a2b3c4d5e6f708192a3b4c5d6e7f809  terraform-provider-caddy_0.2.0_manifest.json\n"
        )

    def test_parse_platforms(self):
        from publish_terralist import parse_platforms
        platforms = parse_platforms(self.sample_sums, "0.2.0", "git.sebclem.fr", "sebclem", "terraform-provider-caddy")
        self.assertEqual(len(platforms), 2)
        self.assertEqual(platforms[0]["os"], "darwin")
        self.assertEqual(platforms[0]["arch"], "amd64")
        self.assertEqual(platforms[0]["shasum"], "c3ab8ff13720e8ad9047dd39466b3c8974e592c2fa383d4a3960714caef0c4f2")
        self.assertEqual(platforms[0]["filename"], "terraform-provider-caddy_0.2.0_darwin_amd64.zip")
        self.assertEqual(platforms[0]["download_url"], "https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v0.2.0/terraform-provider-caddy_0.2.0_darwin_amd64.zip")

    def test_build_payload(self):
        from publish_terralist import build_payload
        payload = build_payload(self.sample_sums, "0.2.0", "git.sebclem.fr", "sebclem", "terraform-provider-caddy")
        self.assertEqual(payload["protocols"], ["6.0"])
        self.assertEqual(payload["shasums"]["url"], "https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v0.2.0/terraform-provider-caddy_0.2.0_SHA256SUMS")
        self.assertEqual(payload["shasums"]["signature_url"], "https://git.sebclem.fr/sebclem/terraform-provider-caddy/releases/download/v0.2.0/terraform-provider-caddy_0.2.0_SHA256SUMS.sig")
        self.assertEqual(len(payload["platforms"]), 2)

    def test_normalize_version(self):
        from publish_terralist import normalize_version
        self.assertEqual(normalize_version("v0.2.0"), "0.2.0")
        self.assertEqual(normalize_version("0.2.0"), "0.2.0")

    @patch("urllib.request.urlopen")
    def test_publish_success(self, mock_urlopen):
        from publish_terralist import publish_to_terralist
        mock_response = MagicMock()
        mock_response.getcode.return_value = 200
        mock_response.read.return_value = b'{"success": true}'
        mock_urlopen.return_value.__enter__.return_value = mock_response

        payload = {"protocols": ["6.0"]}
        publish_to_terralist("https://terralist.sebclem.fr", "secret-key", "sebclem", "caddy", "0.2.0", payload)
        self.assertTrue(mock_urlopen.called)

    @patch("urllib.request.urlopen")
    def test_publish_failure(self, mock_urlopen):
        from publish_terralist import publish_to_terralist
        import urllib.error
        mock_urlopen.side_effect = urllib.error.HTTPError("https://...", 400, "Bad Request", {}, None)

        payload = {"protocols": ["6.0"]}
        with self.assertRaises(RuntimeError):
            publish_to_terralist("https://terralist.sebclem.fr", "secret-key", "sebclem", "caddy", "0.2.0", payload)

if __name__ == "__main__":
    unittest.main()
