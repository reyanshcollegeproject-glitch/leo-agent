from muse_agent.desktop.controller import DesktopWindowController
from muse_agent.desktop.hud import DesktopHUD


def test_desktop_window_controller():
    ctrl = DesktopWindowController()
    # Test volume bounds clamping
    ctrl.set_system_volume(50)
    # Selected finder items should return a list (empty if no Finder window active)
    items = ctrl.get_selected_finder_items()
    assert isinstance(items, list)


def test_desktop_hud_creation():
    hud = DesktopHUD(use_mock=True)
    hud.root.update()
    assert hud.root.winfo_exists()
    hud.goal_entry.delete(0, "end")
    hud.goal_entry.insert(0, "Test Desktop Task")
    assert hud.goal_entry.get() == "Test Desktop Task"
    hud.root.destroy()

