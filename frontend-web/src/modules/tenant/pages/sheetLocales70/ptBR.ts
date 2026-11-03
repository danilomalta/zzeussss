import {mergeLocales} from '@univerjs/presets';
import part0 from '@univerjs/preset-sheets-core/locales/pt-BR';
import part1 from '@univerjs/preset-sheets-conditional-formatting/locales/pt-BR';
import part2 from '@univerjs/preset-sheets-data-validation/locales/pt-BR';
import part3 from '@univerjs/preset-sheets-filter/locales/pt-BR';
import part4 from '@univerjs/preset-sheets-sort/locales/pt-BR';
export default {locale:'ptBR',messages:mergeLocales(part0,part1,part2,part3,part4)};
