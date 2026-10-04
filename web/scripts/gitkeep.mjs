// The build output directory is git-ignored except for this placeholder, which Go's embed needs.
import { writeFileSync } from 'node:fs';

writeFileSync(new URL('../../internal/webui/dist/.gitkeep', import.meta.url), '');
