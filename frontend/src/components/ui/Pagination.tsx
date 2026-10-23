import { ChevronLeft, ChevronRight } from 'lucide-react'
import { buttonClass } from '../../lib/format'

/** Footer with the total count and previous/next buttons. */
export function Pagination({
  page,
  totalPages,
  totalItems,
  noun = 'item',
  plural = `${noun}s`,
  onPage,
}: {
  page: number
  totalPages: number
  totalItems: number
  noun?: string
  plural?: string
  onPage: (page: number) => void
}) {
  if (totalItems === 0) return null
  return (
    <div className="flex items-center justify-between border-t border-gray-200 px-4 py-2.5 text-sm text-gray-500">
      <span>
        {totalItems.toLocaleString()} {totalItems === 1 ? noun : plural}
      </span>
      {totalPages > 1 && (
        <div className="flex items-center gap-2">
          <button type="button" className={buttonClass.ghost} disabled={page <= 1} onClick={() => onPage(page - 1)} aria-label="Previous page">
            <ChevronLeft className="h-4 w-4" />
          </button>
          <span className="tabular-nums">
            {page} / {totalPages}
          </span>
          <button type="button" className={buttonClass.ghost} disabled={page >= totalPages} onClick={() => onPage(page + 1)} aria-label="Next page">
            <ChevronRight className="h-4 w-4" />
          </button>
        </div>
      )}
    </div>
  )
}
