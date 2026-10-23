import { createLazyFileRoute } from '@tanstack/react-router'
import { PlaylistLibrary } from '../../components/PlaylistLibrary'
import { Card, Page } from '../../components/ui'

function SpotifyPlaylists() {
  return (
    <Page
      title="Spotify playlists"
      width="narrow"
      description="Everything in the connected Spotify account. Back up any playlist here; synced ones show their pair."
    >
      <Card className="p-4">
        <PlaylistLibrary service="spotify" />
      </Card>
    </Page>
  )
}

export const Route = createLazyFileRoute('/settings/spotify')({
  component: SpotifyPlaylists,
})
