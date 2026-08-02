import { h } from '../lib/dom';

export function renderSparkline(values: number[], strokeColor: string = '#6366f1'): HTMLCanvasElement {
  const width = 100;
  const height = 24;

  const canvas = h('canvas', {
    className: 'sparkline-canvas',
    width: width * window.devicePixelRatio,
    height: height * window.devicePixelRatio,
    style: { width: `${width}px`, height: `${height}px` },
  }) as HTMLCanvasElement;

  const ctx = canvas.getContext('2d');
  if (!ctx) return canvas;

  ctx.scale(window.devicePixelRatio, window.devicePixelRatio);

  if (values.length < 2) return canvas;

  let min = Math.min(...values);
  let max = Math.max(...values);
  if (min === max) {
    min = 0;
    max = max === 0 ? 100 : max * 1.2;
  }

  ctx.clearRect(0, 0, width, height);
  ctx.beginPath();

  const step = width / (values.length - 1);
  values.forEach((v, i) => {
    const x = i * step;
    const y = height - ((v - min) / (max - min)) * (height - 4) - 2;
    if (i === 0) {
      ctx.moveTo(x, y);
    } else {
      ctx.lineTo(x, y);
    }
  });

  ctx.strokeStyle = strokeColor;
  ctx.lineWidth = 1.5;
  ctx.stroke();

  return canvas;
}
