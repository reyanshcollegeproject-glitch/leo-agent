from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import List, Optional


class SubGoalStatus(str, Enum):
    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    COMPLETED = "completed"
    FAILED = "failed"
    SKIPPED = "skipped"


@dataclass
class SubGoal:
    id: int
    description: str
    status: SubGoalStatus = SubGoalStatus.PENDING
    notes: Optional[str] = None


@dataclass
class ExecutionPlan:
    goal: str
    sub_goals: List[SubGoal] = field(default_factory=list)
    current_index: int = 0

    @property
    def current_subgoal(self) -> Optional[SubGoal]:
        if 0 <= self.current_index < len(self.sub_goals):
            return self.sub_goals[self.current_index]
        return None

    def mark_current_completed(self, notes: Optional[str] = None) -> None:
        if self.current_subgoal:
            self.current_subgoal.status = SubGoalStatus.COMPLETED
            self.current_subgoal.notes = notes
            self.current_index += 1
            if self.current_index < len(self.sub_goals):
                self.sub_goals[self.current_index].status = SubGoalStatus.IN_PROGRESS

    def mark_current_failed(self, notes: Optional[str] = None) -> None:
        if self.current_subgoal:
            self.current_subgoal.status = SubGoalStatus.FAILED
            self.current_subgoal.notes = notes

    def is_finished(self) -> bool:
        return self.current_index >= len(self.sub_goals) or all(
            sg.status in (SubGoalStatus.COMPLETED, SubGoalStatus.SKIPPED) for sg in self.sub_goals
        )


class HierarchicalPlanner:
    """
    Decomposes complex, multi-step instructions into verified sub-goals,
    preventing context drift in long-horizon autonomous tasks.
    """

    def decompose(self, goal: str) -> ExecutionPlan:
        """
        Creates an initial sub-goal breakdown based on task intent.
        For simple tasks, generates a direct tactical plan.
        """
        sub_goals: List[SubGoal] = []
        goal_lower = goal.lower()

        # Check for multi-app workflows
        if "and" in goal_lower or "then" in goal_lower:
            parts = [p.strip() for p in goal.replace(" then ", " and ").split(" and ") if p.strip()]
            for idx, part in enumerate(parts):
                sub_goals.append(SubGoal(id=idx + 1, description=part))
        else:
            # Single composite goal
            sub_goals.append(SubGoal(id=1, description=f"Execute: {goal}"))

        if sub_goals:
            sub_goals[0].status = SubGoalStatus.IN_PROGRESS

        return ExecutionPlan(goal=goal, sub_goals=sub_goals)
