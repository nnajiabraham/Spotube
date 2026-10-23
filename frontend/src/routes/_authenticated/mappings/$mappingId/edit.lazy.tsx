import { createLazyFileRoute, Link, useNavigate, useParams } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftRight, Ban } from 'lucide-react'
import { mappingsAPI } from '../../../../lib/api/index'
import { MappingOptions, type MappingOptionsValue } from '../../../../components/MappingOptions'
import { MappingSyncStatus, SyncNowButton } from '../../../../components/MappingSync'
import { Card, Page, ServiceIcon } from '../../../../components/ui'
import { buttonClass } from '../../../../lib/format'
import { useSyncNow } from '../../../../lib/useSyncNow'

function EditMapping() {
  const { mappingId } = useParams({ from: '/_authenticated/mappings/$mappingId/edit' })
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const sync = useSyncNow()
  const [options, setOptions] = useState<MappingOptionsValue | null>(null)

  const { data: mapping, isLoading, error } = useQuery({
    queryKey: ['mappings', mappingId],
    queryFn: () => mappingsAPI.getOne(mappingId),
  })

  useEffect(() => {
    if (mapping && !options) {
      setOptions({
        sync_tracks: mapping.sync_tracks,
        sync_name: mapping.sync_name,
        interval_minutes: mapping.interval_minutes,
      })
    }
  }, [mapping, options])

  const save = useMutation({
    mutationFn: (value: MappingOptionsValue) => mappingsAPI.update(mappingId, value),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mappings'] })
      navigate({ to: '/mappings' })
    },
  })

  if (isLoading || (mapping && !options)) {
    return (
      <Page title="Edit mapping" width="narrow">
        <p className="text-sm text-gray-500">Loading…</p>
      </Page>
    )
  }

  if (error || !mapping || !options) {
    return (
      <Page title="Edit mapping" width="narrow">
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
          Mapping not found{error ? `: ${error.message}` : '.'}
        </div>
      </Page>
    )
  }

  return (
    <Page title="Edit mapping" width="narrow" actions={<SyncNowButton mappingId={mapping.id} sync={sync} />}>
      <Card className="mb-4 p-5">
        <div className="flex flex-wrap items-center gap-3 font-medium text-gray-900">
          <span className="flex items-center gap-2">
            <ServiceIcon service="spotify" className="h-5 w-5" />
            {mapping.spotify_playlist_name || mapping.spotify_playlist_id}
          </span>
          <ArrowLeftRight className="h-4 w-4 text-gray-400" />
          <span className="flex items-center gap-2">
            <ServiceIcon service="youtube" className="h-5 w-5" />
            {mapping.youtube_playlist_name || mapping.youtube_playlist_id}
          </span>
        </div>
        <div className="mt-3 text-sm">
          <MappingSyncStatus mapping={mapping} />
        </div>
        {mapping.last_sync_error && (
          <p className="mt-2 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">{mapping.last_sync_error}</p>
        )}
      </Card>

      <form
        onSubmit={(e) => {
          e.preventDefault()
          save.mutate(options)
        }}
      >
        <Card className="p-5">
          <MappingOptions value={options} onChange={setOptions} />
        </Card>

        {save.error && (
          <div className="mt-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
            {save.error.message}
          </div>
        )}

        <div className="mt-6 flex flex-wrap items-center justify-between gap-3">
          <Link
            to="/mappings/$mappingId/blacklist"
            params={{ mappingId }}
            className="inline-flex items-center gap-1.5 text-sm text-gray-600 hover:text-gray-900"
          >
            <Ban className="h-4 w-4" /> Blocked tracks
          </Link>
          <div className="flex gap-3">
            <Link to="/mappings" className={buttonClass.secondary}>
              Cancel
            </Link>
            <button type="submit" className={buttonClass.primary} disabled={save.isPending}>
              {save.isPending ? 'Saving…' : 'Save changes'}
            </button>
          </div>
        </div>
      </form>
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/$mappingId/edit')({
  component: EditMapping,
})
