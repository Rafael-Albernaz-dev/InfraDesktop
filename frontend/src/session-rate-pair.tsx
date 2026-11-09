import type { PPRate } from './presentation-pppoe-types';
import { exactRate, formatAtScale, networkScale } from './network-format';
import { RateValue } from './pppoe-read-views';

export function SessionRatePair({
  download,
  upload,
}: {
  download?: PPRate;
  upload?: PPRate;
}) {
  if (
    !download ||
    !upload ||
    download.min !== download.max ||
    upload.min !== upload.max ||
    !Number.isFinite(download.min) ||
    !Number.isFinite(upload.min) ||
    download.min < 0 ||
    upload.min < 0
  )
    return (
      <>
        <RateValue rate={download} /> / <RateValue rate={upload} />
      </>
    );
  const scale = networkScale(Math.max(download.max, upload.max), 'rate');
  return (
    <span title={`${exactRate(download.min)} / ${exactRate(upload.min)}`}>
      {formatAtScale(download.min, scale, false)} /{' '}
      {formatAtScale(upload.min, scale, false)} {scale.unit}
    </span>
  );
}
