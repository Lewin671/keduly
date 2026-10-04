// Creating a project, or changing its name, colour and area.
import type { JSX } from 'preact';
import { useState } from 'preact/hooks';
import * as api from '../api/client';
import type { Project, ProjectColor } from '../api/types';
import { t } from '../i18n';
import { navigate } from '../state/route';
import { areas, write } from '../state/store';
import { Dialog } from './Popover';

export const COLORS: readonly ProjectColor[] = ['blue', 'indigo', 'orange', 'teal', 'green', 'pink', 'purple', 'brown'];

const NEW_AREA = '@new';

export function ProjectDialog({ project, onClose }: { project?: Project; onClose: () => void }): JSX.Element {
  const [name, setName] = useState(project?.name ?? '');
  // No colour chosen: the server picks the least used one.
  const [color, setColor] = useState<ProjectColor | null>(project?.color ?? null);
  const [area, setArea] = useState(project?.area_id ?? '');
  const [areaName, setAreaName] = useState('');
  const [busy, setBusy] = useState(false);

  const save = async () => {
    const title = name.trim();
    if (!title || busy || (area === NEW_AREA && !areaName.trim())) return;
    setBusy(true);
    const areaId = area === NEW_AREA ? (await write(api.createArea(areaName.trim())))?.id : area || null;
    if (areaId === undefined) return setBusy(false);
    const body = { name: title, area_id: areaId, ...(color ? { color } : {}) };
    const saved = await write(project ? api.updateProject(project.id, body) : api.createProject(body));
    setBusy(false);
    if (!saved) return;
    onClose();
    if (!project) navigate({ mode: 'items', list: `p:${saved.id}` });
  };

  return (
    <Dialog onClose={onClose} label={project ? t('project.edit') : t('project.new')} class="small">
      <form class="form" onSubmit={event => { event.preventDefault(); void save(); }}>
        <div class="sheet-h">
          <button type="button" onClick={onClose}>{t('common.cancel')}</button>
          <b>{project ? t('project.edit') : t('project.new')}</b>
          <button type="submit" disabled={!name.trim() || busy}>{project ? t('common.save') : t('common.create')}</button>
        </div>
        <label class="lbl">{t('project.name')}
          <input class="fld" value={name} maxLength={200} autoFocus onInput={event => setName(event.currentTarget.value)} />
        </label>
        <div class="lbl">{t('project.color')}
          <div class="swatches" role="radiogroup" aria-label={t('project.color')}>
            {COLORS.map(c => (
              <button type="button" role="radio" aria-checked={c === color} aria-label={t(`color.${c}`)} class={c === color ? 'sw on' : 'sw'} style={{ '--c': `var(--${c})` }} onClick={() => setColor(c)} />
            ))}
          </div>
        </div>
        <label class="lbl">{t('project.area')}
          <select class="fld" value={area} onChange={event => setArea(event.currentTarget.value)}>
            <option value="">{t('area.none')}</option>
            {areas.value.map(a => <option value={a.id}>{a.name}</option>)}
            <option value={NEW_AREA}>{t('area.newOption')}</option>
          </select>
        </label>
        {area === NEW_AREA && (
          <label class="lbl">{t('area.name')}
            <input class="fld" value={areaName} maxLength={100} onInput={event => setAreaName(event.currentTarget.value)} />
          </label>
        )}
      </form>
    </Dialog>
  );
}
