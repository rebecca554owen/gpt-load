import { ref } from "vue";

export function useChartAnimation(plotWidth: number, plotHeight: number) {
  const animationProgress = ref(0);
  const animatedStroke = ref("0");
  const animatedOffset = ref("0");

  const startAnimation = () => {
    const totalLength = plotWidth + plotHeight;
    animatedStroke.value = `${totalLength}`;
    animatedOffset.value = `${totalLength}`;

    let start = 0;
    const animate = (timestamp: number) => {
      if (!start) {
        start = timestamp;
      }
      const progress = Math.min((timestamp - start) / 1500, 1);

      animatedOffset.value = `${totalLength * (1 - progress)}`;
      animationProgress.value = progress;

      if (progress < 1) {
        requestAnimationFrame(animate);
      }
    };
    requestAnimationFrame(animate);
  };

  return {
    animationProgress,
    animatedStroke,
    animatedOffset,
    startAnimation,
  };
}
