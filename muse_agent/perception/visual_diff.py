from __future__ import annotations

from dataclasses import dataclass
from typing import Optional, Tuple
from PIL import Image
import numpy as np


@dataclass
class VisualDiffResult:
    has_changed: bool
    diff_ratio: float
    changed_bbox: Optional[Tuple[int, int, int, int]]  # (x1, y1, x2, y2) in normalized [0, 1000]
    is_near_action: bool
    description: str


class VisualVerifier:
    """
    Closed-loop visual verification engine.
    Compares pre-action and post-action screen states to detect whether
    the OS or application responded to the agent's command.
    """

    def __init__(self, threshold: float = 0.003):
        self.threshold = threshold

    def compare(
        self,
        before: Image.Image,
        after: Image.Image,
        action_coord: Optional[Tuple[int, int]] = None
    ) -> VisualDiffResult:
        """
        Compare before and after screenshots.
        action_coord: Optional (norm_x, norm_y) where the action was executed.
        """
        # Ensure identical sizes
        if before.size != after.size:
            after = after.resize(before.size)

        w, h = before.size
        # Fast difference calculation with numpy
        arr_b = np.array(before, dtype=np.int16)
        arr_a = np.array(after, dtype=np.int16)

        # Absolute channel differences
        diff = np.abs(arr_a - arr_b)
        # Pixel is considered changed if Euclidean-ish color distance > 25
        pixel_diff = np.max(diff, axis=2) > 25

        changed_count = int(np.count_nonzero(pixel_diff))
        total_pixels = w * h
        diff_ratio = changed_count / float(total_pixels)

        has_changed = diff_ratio >= self.threshold

        if not has_changed:
            return VisualDiffResult(
                has_changed=False,
                diff_ratio=diff_ratio,
                changed_bbox=None,
                is_near_action=False,
                description=f"No significant visual change detected (delta: {diff_ratio * 100:.2f}%)"
            )

        # Find bounding box of changed area
        ys, xs = np.nonzero(pixel_diff)
        y_min, y_max = int(np.min(ys)), int(np.max(ys))
        x_min, x_max = int(np.min(xs)), int(np.max(xs))

        norm_bbox = (
            int((x_min / float(w)) * 1000),
            int((y_min / float(h)) * 1000),
            int((x_max / float(w)) * 1000),
            int((y_max / float(h)) * 1000),
        )

        is_near = False
        if action_coord is not None:
            ax, ay = action_coord
            # Check if action point is inside or close to the changed region (margin = 150 norm units)
            bx1, by1, bx2, by2 = norm_bbox
            margin = 150
            if (bx1 - margin <= ax <= bx2 + margin) and (by1 - margin <= ay <= by2 + margin):
                is_near = True

        desc = (
            f"Visual change confirmed: {diff_ratio * 100:.2f}% of screen updated. "
            f"Region: x=[{norm_bbox[0]}, {norm_bbox[2]}], y=[{norm_bbox[1]}, {norm_bbox[3]}]."
        )
        if is_near:
            desc += " Change occurred directly adjacent to action target."

        return VisualDiffResult(
            has_changed=True,
            diff_ratio=diff_ratio,
            changed_bbox=norm_bbox,
            is_near_action=is_near,
            description=desc
        )
