import importlib.util
import pathlib
import tempfile
import unittest


PATH = pathlib.Path(__file__).with_name(
    "fetch_auto_source_bundle_objects.py"
)
SPEC = importlib.util.spec_from_file_location(
    "fetch_auto_source_bundle_objects",
    PATH,
)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class FetchAutoSourceBundleObjectsTest(unittest.TestCase):
    def test_mime_compatibility_is_narrow_and_directional(self):
        self.assertTrue(
            MODULE.mime_compatible(
                "image/vnd.adobe.photoshop",
                "application/octet-stream",
            )
        )
        self.assertTrue(
            MODULE.mime_compatible(
                "IMAGE/JPEG; charset=binary",
                "image/jpeg",
            )
        )
        self.assertFalse(
            MODULE.mime_compatible(
                "application/octet-stream",
                "image/jpeg",
            )
        )
        self.assertFalse(
            MODULE.mime_compatible(
                "image/png",
                "image/jpeg",
            )
        )

    def test_safe_target_rejects_escape_and_symlink(self):
        with tempfile.TemporaryDirectory() as raw:
            root = pathlib.Path(raw).resolve()
            target = MODULE.safe_target(root, "tasks/1/source.psd")
            self.assertEqual(
                target,
                root / "tasks" / "1" / "source.psd",
            )
            with self.assertRaisesRegex(ValueError, "unsafe"):
                MODULE.safe_target(root, "../escape")
            symlink = root / "linked"
            symlink.symlink_to(root / "tasks", target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "escaped|symlink"):
                MODULE.safe_target(root, "linked/object.psd")


if __name__ == "__main__":
    unittest.main()
