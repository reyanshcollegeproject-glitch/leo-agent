from muse_agent.perception.screen import ScreenPerceiver, ScreenInfo


def test_coordinate_normalization_roundtrip():
    perceiver = ScreenPerceiver()
    # Mock known geometry
    perceiver._screen_info = ScreenInfo(
        logical_width=1440,
        logical_height=900,
        physical_width=2880,
        physical_height=1800,
        scale_factor=2.0
    )

    # Center of screen (500, 500)
    lx, ly = perceiver.normalized_to_logical(500, 500)
    assert lx == 720
    assert ly == 450

    nx, ny = perceiver.logical_to_normalized(720, 450)
    assert nx == 500
    assert ny == 500

    # Bounds clamping
    lx_max, ly_max = perceiver.normalized_to_logical(1200, 1500)
    assert lx_max == 1439
    assert ly_max == 899


def test_capture_base64():
    perceiver = ScreenPerceiver()
    b64, img = perceiver.capture_base64(max_width=800)
    assert isinstance(b64, str)
    assert len(b64) > 100
    assert img.width <= 800
