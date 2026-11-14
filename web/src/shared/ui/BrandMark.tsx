type BrandMarkProps = {
  size?: number
}

/** 统一品牌标识：胶片外框与中心播放三角 */
export function BrandMark({ size = 20 }: BrandMarkProps) {
  return (
    <svg
      aria-hidden="true"
      fill="none"
      height={size}
      viewBox="0 0 32 32"
      width={size}
    >
      <path
        d="M8.5 4.5h15A2.5 2.5 0 0 1 26 7v18a2.5 2.5 0 0 1-2.5 2.5h-15A2.5 2.5 0 0 1 6 25V7a2.5 2.5 0 0 1 2.5-2.5Zm3.5 3.75a1 1 0 0 0-1 1v13.5a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V9.25a1 1 0 0 0-1-1h-8ZM8.5 9.25a1.1 1.1 0 1 0 0-2.2 1.1 1.1 0 0 0 0 2.2Zm0 7.85a1.1 1.1 0 1 0 0-2.2 1.1 1.1 0 0 0 0 2.2Zm0 7.85a1.1 1.1 0 1 0 0-2.2 1.1 1.1 0 0 0 0 2.2Zm15-15.7a1.1 1.1 0 1 0 0-2.2 1.1 1.1 0 0 0 0 2.2Zm0 7.85a1.1 1.1 0 1 0 0-2.2 1.1 1.1 0 0 0 0 2.2Zm0 7.85a1.1 1.1 0 1 0 0-2.2 1.1 1.1 0 0 0 0 2.2Z"
        fill="currentColor"
        fillRule="evenodd"
      />
      <path
        d="M13.8 12.65a.7.7 0 0 1 1.07-.59l5.3 3.35a.7.7 0 0 1 0 1.18l-5.3 3.35a.7.7 0 0 1-1.07-.59v-6.7Z"
        fill="currentColor"
        opacity="0.92"
      />
    </svg>
  )
}
