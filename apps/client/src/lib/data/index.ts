import { USE_FAKE_DATA } from '../env';
import { fake } from './fake';
import { real } from './real';
import type { DataSource } from './types';

/**
 * The one place that decides where data comes from. PUBLIC_USE_FAKE_DATA=true (the default)
 * uses @yari/fixtures. Anything else goes through @yari/api-client.
 */
export const data: DataSource = USE_FAKE_DATA ? fake : real;
export * from './types';
