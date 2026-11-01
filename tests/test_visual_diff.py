from PIL import Image, ImageDraw
from muse_agent.perception.visual_diff import VisualVerifier


def test_visual_diff_identical():
    img = Image.new("RGB", (400, 300), color=(50, 50, 50))
    verifier = VisualVerifier(threshold=0.01)
    res = verifier.compare(img, img)
    assert not res.has_changed
    assert res.diff_ratio == 0.0


def test_visual_diff_with_change():
    before = Image.new("RGB", (400, 300), color=(50, 50, 50))
    after = before.copy()
    draw = ImageDraw.Draw(after)
    # Draw significant change: white box at (100, 100) -> (250, 250)
    draw.rectangle([(100, 100), (250, 250)], fill=(255, 255, 255))

    verifier = VisualVerifier(threshold=0.01)
    res = verifier.compare(before, after, action_coord=(300, 400))
    assert res.has_changed
    assert res.diff_ratio > 0.05
    assert res.changed_bbox is not None
