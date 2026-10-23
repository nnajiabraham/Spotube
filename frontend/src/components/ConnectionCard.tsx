import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { api, oauthAPI } from '../lib/api'
import { shouldRetryConnectionCheck } from '../lib/connectionCheckQuery'
import { Card, ServiceIcon } from './ui'
import { buttonClass, serviceName } from '../lib/format'

type Service = 'spotify' | 'youtube'

const probes: Record<Service, () => Promise<unknown>> = {
  spotify: () => api.getSpotifyPlaylists({ limit: 1 }),
  youtube: () => api.getYouTubePlaylists(),
}

/** Shows whether an OAuth provider is usable, by probing its playlist endpoint. */
export function ConnectionCard({ service }: { service: Service }) {
  const name = serviceName(service)
  const { isLoading, error } = useQuery({
    queryKey: [`${service}-connection`],
    queryFn: probes[service],
    retry: shouldRetryConnectionCheck,
    retryDelay: (attemptIndex) => Math.min(500 * 2 ** attemptIndex, 2000),
  })
  const connect = () => {
    window.location.href = oauthAPI[service].getLoginURL()
  }

  return (
    <Card className="flex items-center gap-4 p-4">
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-gray-50 ring-1 ring-gray-200">
        <ServiceIcon service={service} className="h-5 w-5" />
      </div>
      {isLoading ? (
        <div className="flex-1 animate-pulse space-y-2" role="status" aria-label="Loading">
          <div className="h-4 w-24 rounded bg-gray-200" />
          <div className="h-3 w-16 rounded bg-gray-200" />
        </div>
      ) : error ? (
        <>
          <div className="min-w-0 flex-1">
            <h3 className="font-medium text-gray-900">{name}</h3>
            <p className="flex items-center gap-1.5 text-sm text-amber-700">
              <span className="h-2 w-2 rounded-full bg-amber-500" />
              Not connected
            </p>
          </div>
          <button type="button" onClick={connect} className={buttonClass.primary}>
            Connect {name}
          </button>
        </>
      ) : (
        <>
          <div className="min-w-0 flex-1">
            <h3 className="font-medium text-gray-900">{name}</h3>
            <p className="flex items-center gap-1.5 text-sm text-green-700">
              <span className="h-2 w-2 rounded-full bg-green-500" />
              Connected
            </p>
          </div>
          <Link to={`/settings/${service}`} className={buttonClass.secondary}>
            Playlists
          </Link>
          <button type="button" onClick={connect} className={buttonClass.ghost} title={`Sign in to ${name} again`}>
            Reconnect
          </button>
        </>
      )}
    </Card>
  )
}
