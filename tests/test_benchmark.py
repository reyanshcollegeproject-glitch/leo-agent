import pytest
from pathlib import Path
from muse_agent.benchmark.runner import BenchmarkRunner
from muse_agent.benchmark.tasks import get_standard_benchmark_suite


@pytest.mark.asyncio
async def test_benchmark_suite_runner(tmp_path: Path):
    tasks = get_standard_benchmark_suite()
    # Test first 2 tasks for fast integration check
    runner = BenchmarkRunner(tasks=tasks[:2], use_mock=True, output_dir=tmp_path)
    res = await runner.run_suite()

    assert res.total_tasks == 2
    assert res.successful_tasks == 2
    assert res.success_rate_percent == 100.0
    assert (tmp_path / "report.md").exists()
    assert (tmp_path / "report.json").exists()
