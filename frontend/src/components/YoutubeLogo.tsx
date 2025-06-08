import React from 'react';

export function YoutubeLogo(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 28 20"
      fill="currentColor"
      aria-hidden="true"
      {...props}
    >
      <path
        fillRule="evenodd"
        d="M27.52 3.03C27.19 1.84 26.24.88 25.05.56 22.86 0 14 0 14 0S5.14 0 2.95.56C1.76.88.81 1.84.48 3.03 0 5.2 0 10 0 10s0 4.8.48 6.97c.33 1.19 1.28 2.15 2.47 2.47C5.14 20 14 20 14 20s8.86 0 11.05-.56c1.19-.32 2.14-1.28 2.47-2.47C28 14.8 28 10 28 10s0-4.8-.48-6.97zM11.2 14V6l7.2 4-7.2 4z"
        clipRule="evenodd"
      />
    </svg>
  );
} 