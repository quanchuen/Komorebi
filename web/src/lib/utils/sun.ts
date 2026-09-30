// web/src/lib/utils/sun.ts
//
// Sunrise and sunset for the time scrubber's ☀︎ / ☾ marks. NOAA's simplified
// algorithm; accurate to a minute or two, which is all a scrubber needs.
const rad = Math.PI / 180;

function julianDay(date: Date): number {
  return date.getTime() / 86_400_000 - 0.5 + 2_440_588;
}

/** Local sunrise and sunset on `date` at (lat, lon), or null in polar night/day. */
export function sunTimes(
  date: Date,
  lat: number,
  lon: number
): { sunrise: Date; sunset: Date } | null {
  // Nearest Julian noon: the algorithm counts days from 2000-01-01 12:00 UT.
  const jd = Math.round(julianDay(date));
  const n = jd - 2_451_545 + 0.0008;
  // Solar noon comes earlier the further east: east-positive longitude.
  const meanSolarNoon = n - lon / 360;
  const meanAnomaly = (357.5291 + 0.98560028 * meanSolarNoon) % 360;
  const centre =
    1.9148 * Math.sin(meanAnomaly * rad) +
    0.02 * Math.sin(2 * meanAnomaly * rad) +
    0.0003 * Math.sin(3 * meanAnomaly * rad);
  const eclipticLon = (meanAnomaly + centre + 180 + 102.9372) % 360;
  const transit =
    2_451_545 +
    meanSolarNoon +
    0.0053 * Math.sin(meanAnomaly * rad) -
    0.0069 * Math.sin(2 * eclipticLon * rad);
  const declination = Math.asin(Math.sin(eclipticLon * rad) * Math.sin(23.4397 * rad));
  const cosHourAngle =
    (Math.sin(-0.833 * rad) - Math.sin(lat * rad) * Math.sin(declination)) /
    (Math.cos(lat * rad) * Math.cos(declination));
  if (cosHourAngle < -1 || cosHourAngle > 1) return null;
  const hourAngle = Math.acos(cosHourAngle) / rad / 360;
  const toDate = (j: number) => new Date((j - 2_440_588 + 0.5) * 86_400_000);
  return { sunrise: toDate(transit - hourAngle), sunset: toDate(transit + hourAngle) };
}
