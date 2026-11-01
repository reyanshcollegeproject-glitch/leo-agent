from __future__ import annotations

import io
import base64
from typing import Dict, List, Tuple, Optional
from PIL import Image, ImageDraw, ImageFont
import numpy as np


class VisualGrounder:
    """
    Set-of-Marks (SoM) and Visual Grid Grounding Engine.
    Transforms raw desktop screenshots into structured, actionable visual spaces
    that eliminate coordinate hallucination and set new benchmarks for precision.
    """

    def __init__(self, grid_step: int = 100):
        self.grid_step = grid_step
        self._font = self._load_font()

    def _load_font(self) -> ImageFont.ImageFont:
        """Attempt to load a clean system TrueType font or fallback to default."""
        font_paths = [
            "/System/Library/Fonts/SFNSMono.ttf",
            "/System/Library/Fonts/Helvetica.ttc",
            "/Library/Fonts/Arial.ttf",
            "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
        ]
        for path in font_paths:
            try:
                return ImageFont.truetype(path, size=12)
            except Exception:
                continue
        return ImageFont.load_default()

    def apply_grid_overlay(self, image: Image.Image) -> Image.Image:
        """
        Overlays a normalized [0, 1000] reference grid onto the screenshot.
        Allows the model to easily triangulate coordinates visually.
        """
        overlay = image.copy()
        draw = ImageDraw.Draw(overlay, "RGBA")
        w, h = overlay.size

        # Grid lines every self.grid_step normalized units
        for norm_coord in range(self.grid_step, 1000, self.grid_step):
            x = int((norm_coord / 1000.0) * w)
            y = int((norm_coord / 1000.0) * h)

            # Vertical grid line (semi-transparent red/gray)
            draw.line([(x, 0), (x, h)], fill=(255, 60, 60, 40), width=1)
            # Horizontal grid line
            draw.line([(0, y), (w, y)], fill=(255, 60, 60, 40), width=1)

            # Draw coordinate markers along top and left edges
            draw.text((x + 2, 4), f"{norm_coord}", fill=(220, 20, 60, 220), font=self._font)
            draw.text((4, y + 2), f"{norm_coord}", fill=(220, 20, 60, 220), font=self._font)

        return overlay

    def detect_ui_candidates(self, image: Image.Image, max_elements: int = 50) -> List[Dict[str, int]]:
        """
        Fast heuristic UI element detector (buttons, inputs, tabs, icons)
        based on edge gradients, high-contrast bounding boxes, and component clustering.
        Returns a list of candidate bounding boxes in normalized [0, 1000] coordinates:
        [{'id': 1, 'x': 250, 'y': 150, 'w': 80, 'h': 30, 'cx': 290, 'cy': 165}, ...]
        """
        w, h = image.size
        # Convert to grayscale numpy array for fast gradient calculation
        gray = np.array(image.convert("L"), dtype=np.uint8)

        # Compute Sobel-like simple gradient
        gx = np.abs(gray[:, 1:] - gray[:, :-1])
        gy = np.abs(gray[1:, :] - gray[:-1, :])

        # Pad to match original dimensions
        edges = np.zeros_like(gray)
        edges[:, :-1] += gx
        edges[:-1, :] += gy

        # Threshold to find significant UI edges
        edge_mask = edges > 20

        # Downsample grid to cluster contiguous active UI regions
        block_size = max(16, min(w, h) // 40)
        grid_h = h // block_size
        grid_w = w // block_size

        candidates = []
        element_id = 1

        for gy_idx in range(grid_h):
            for gx_idx in range(grid_w):
                y_start = gy_idx * block_size
                y_end = min(y_start + block_size, h)
                x_start = gx_idx * block_size
                x_end = min(x_start + block_size, w)

                region = edge_mask[y_start:y_end, x_start:x_end]
                density = float(np.mean(region))

                # Active UI density threshold - sensitive to subtle buttons, text, and icons
                if 0.03 <= density <= 0.95:
                    cx = (x_start + x_end) // 2
                    cy = (y_start + y_end) // 2

                    norm_cx = int((cx / float(w)) * 1000)
                    norm_cy = int((cy / float(h)) * 1000)
                    norm_w = int((block_size / float(w)) * 1000)
                    norm_h = int((block_size / float(h)) * 1000)

                    candidates.append({
                        "id": element_id,
                        "x": norm_cx - norm_w // 2,
                        "y": norm_cy - norm_h // 2,
                        "w": norm_w,
                        "h": norm_h,
                        "cx": norm_cx,
                        "cy": norm_cy
                    })
                    element_id += 1
                    if len(candidates) >= max_elements:
                        return candidates

        return candidates

    def apply_set_of_marks(
        self,
        image: Image.Image,
        candidates: Optional[List[Dict[str, int]]] = None
    ) -> Tuple[Image.Image, Dict[int, Tuple[int, int]]]:
        """
        Applies numbered Set-of-Marks tags onto the screenshot.
        Returns:
            annotated_image: Image with tags [1], [2], [3]
            mark_to_coords: Dictionary mapping tag ID -> (normalized_cx, normalized_cy)
        """
        if candidates is None:
            candidates = self.detect_ui_candidates(image)

        annotated = image.copy()
        draw = ImageDraw.Draw(annotated, "RGBA")
        w, h = annotated.size
        mark_to_coords: Dict[int, Tuple[int, int]] = {}

        for elem in candidates:
            mark_id = elem["id"]
            norm_cx, norm_cy = elem["cx"], elem["cy"]
            mark_to_coords[mark_id] = (norm_cx, norm_cy)

            # Map to image pixels
            px = int((norm_cx / 1000.0) * w)
            py = int((norm_cy / 1000.0) * h)

            # Draw small tag badge
            tag_text = str(mark_id)
            tag_w = max(18, len(tag_text) * 8 + 6)
            tag_h = 16

            box_x1 = max(0, px - tag_w // 2)
            box_y1 = max(0, py - tag_h // 2)
            box_x2 = min(w, box_x1 + tag_w)
            box_y2 = min(h, box_y1 + tag_h)

            # Distinctive neon badge for high visual saliency
            draw.rectangle(
                [(box_x1, box_y1), (box_x2, box_y2)],
                fill=(18, 24, 38, 230),
                outline=(0, 230, 255, 255),
                width=1
            )
            draw.text((box_x1 + 3, box_y1 + 1), tag_text, fill=(255, 255, 255, 255), font=self._font)

        return annotated, mark_to_coords

    def annotate_action(
        self,
        image: Image.Image,
        action_name: str,
        norm_x: int,
        norm_y: int
    ) -> Image.Image:
        """
        Draws action indicators (e.g. click ring, crosshair) on screenshot
        for telemetry, video replay, and the real-time web observer UI.
        """
        annotated = image.copy()
        draw = ImageDraw.Draw(annotated, "RGBA")
        w, h = annotated.size

        px = int((norm_x / 1000.0) * w)
        py = int((norm_y / 1000.0) * h)
        radius = 16

        # Outer pulsing rings
        draw.ellipse(
            [(px - radius, py - radius), (px + radius, py + radius)],
            outline=(255, 50, 50, 230),
            width=3
        )
        draw.ellipse(
            [(px - 4, py - 4), (px + 4, py + 4)],
            fill=(255, 255, 0, 240)
        )
        # Action label banner
        banner_text = f"{action_name} ({norm_x}, {norm_y})"
        draw.rectangle([(px + 12, py - 10), (px + 12 + len(banner_text) * 8, py + 12)], fill=(0, 0, 0, 200))
        draw.text((px + 16, py - 8), banner_text, fill=(0, 255, 128, 255), font=self._font)

        return annotated
