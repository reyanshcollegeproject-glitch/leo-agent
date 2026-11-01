from PIL import Image, ImageDraw
from muse_agent.perception.grounding import VisualGrounder


def create_synthetic_gui():
    img = Image.new("RGB", (1000, 800), color=(240, 240, 245))
    draw = ImageDraw.Draw(img)

    # Simulated button 1
    draw.rectangle([(100, 100), (250, 150)], fill=(0, 122, 255), outline=(0, 80, 200))
    draw.text((120, 120), "Submit Button", fill=(255, 255, 255))

    # Simulated input field
    draw.rectangle([(300, 100), (600, 150)], fill=(255, 255, 255), outline=(150, 150, 150))
    draw.text((310, 120), "Enter search query...", fill=(100, 100, 100))

    # Simulated card
    draw.rectangle([(100, 250), (800, 500)], fill=(255, 255, 255), outline=(200, 200, 200))
    draw.text((120, 270), "Card Title", fill=(20, 20, 20))

    return img


def test_ui_candidate_detection():
    img = create_synthetic_gui()
    grounder = VisualGrounder()
    candidates = grounder.detect_ui_candidates(img)
    assert len(candidates) > 0
    # Every candidate should have normalized coords [0, 1000]
    for c in candidates:
        assert 0 <= c["cx"] <= 1000
        assert 0 <= c["cy"] <= 1000
        assert c["id"] >= 1


def test_set_of_marks_overlay():
    img = create_synthetic_gui()
    grounder = VisualGrounder()
    annotated, mark_coords = grounder.apply_set_of_marks(img)
    assert annotated.size == img.size
    assert len(mark_coords) > 0
    for mark_id, (nx, ny) in mark_coords.items():
        assert isinstance(mark_id, int)
        assert 0 <= nx <= 1000
        assert 0 <= ny <= 1000


def test_grid_overlay():
    img = create_synthetic_gui()
    grounder = VisualGrounder(grid_step=200)
    overlaid = grounder.apply_grid_overlay(img)
    assert overlaid.size == img.size


def test_action_annotation():
    img = create_synthetic_gui()
    grounder = VisualGrounder()
    action_img = grounder.annotate_action(img, "click", 500, 500)
    assert action_img.size == img.size

