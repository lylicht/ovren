import pawnIoLicense from '../../../../src-tauri/crates/mangodisk-privileged/vendors/pawnio/modules/0.2.11/COPYING?raw';
import sfStudioLicense from './licenses/sfstudio-apache-2.0.txt?raw';
import ovrenNetLicense from '../../../../ovrennet/LICENSE?raw';
import ovrenNetDependencies from '../../../../ovrennet/licenses/dependencies.txt?raw';
import ovrenNetWindowsLicense from '../../../../ovrennet/licenses/wintun.txt?raw';

export interface ThirdPartyDocument {
  readonly id: string;
  readonly titleKey: string;
  readonly actionKey: string;
  readonly format: 'text' | 'markdown';
  readonly content: string;
}

export interface ThirdPartyComponent {
  readonly id: string;
  readonly nameKey: string;
  readonly version: string;
  readonly author: string;
  readonly licenseKey: string;
  readonly descriptionKey: string;
  readonly projectUrl: string;
  readonly documents: readonly ThirdPartyDocument[];
}

// Canonical documents are bundled only with the lazy license dialog. Component
// metadata keeps ownership and project links independent of the viewer layout.
export const THIRD_PARTY_COMPONENTS: readonly ThirdPartyComponent[] = [
  {
    id: 'pawnio-modules',
    nameKey: 'thirdPartyLicenses.components.pawnioModules.name',
    version: '0.2.11',
    author: 'namazso',
    licenseKey: 'thirdPartyLicenses.components.pawnioModules.license',
    descriptionKey: 'thirdPartyLicenses.components.pawnioModules.description',
    projectUrl: 'https://github.com/namazso/PawnIO.Modules',
    documents: [
      {
        id: 'license',
        titleKey: 'thirdPartyLicenses.components.pawnioModules.license',
        actionKey: 'thirdPartyLicenses.readLicense',
        format: 'text',
        content: pawnIoLicense,
      },
    ],
  },
  {
    id: 'pawnio-installer',
    nameKey: 'thirdPartyLicenses.components.pawnioInstaller.name',
    version: '2.2.0',
    author: 'namazso',
    licenseKey: 'thirdPartyLicenses.components.pawnioInstaller.license',
    descriptionKey: 'thirdPartyLicenses.components.pawnioInstaller.description',
    projectUrl: 'https://github.com/namazso/PawnIO.Setup',
    documents: [],
  },
  {
    id: 'sfstudio-symbols',
    nameKey: 'thirdPartyLicenses.components.sfStudioSymbols.name',
    version: 'main@cdd06d88',
    author: 'Licht and SFStudio contributors; Apple Inc. (SF Symbols)',
    licenseKey: 'thirdPartyLicenses.components.sfStudioSymbols.license',
    descriptionKey: 'thirdPartyLicenses.components.sfStudioSymbols.description',
    projectUrl: 'https://github.com/lylicht/SFStudio',
    documents: [
      {
        id: 'license',
        titleKey: 'thirdPartyLicenses.components.sfStudioSymbols.license',
        actionKey: 'thirdPartyLicenses.readLicense',
        format: 'text',
        content: sfStudioLicense,
      },
      {
        id: 'apple-terms',
        titleKey: 'thirdPartyLicenses.components.sfStudioSymbols.appleTerms',
        actionKey: 'thirdPartyLicenses.readLicense',
        format: 'markdown',
        content:
          'SF Symbols are Apple’s property. Ovren uses them only as interface icons, not as its logo, app icon, or trademark. Read [Apple’s SF Symbols guidance](https://developer.apple.com/design/human-interface-guidelines/sf-symbols/).',
      },
    ],
  },
  {
    id: 'ovrennet',
    nameKey: 'ovrenNet.licenseName',
    version: '1.0.0',
    author: 'Licht (Ovren integration)',
    licenseKey: 'ovrenNet.licenseType',
    descriptionKey: 'ovrenNet.licenseDescription',
    projectUrl: 'https://github.com/lylicht/ovren',
    documents: [
      {
        id: 'license',
        titleKey: 'ovrenNet.licenseSourceDocument',
        actionKey: 'thirdPartyLicenses.readLicense',
        format: 'text',
        content: ovrenNetLicense,
      },
      {
        id: 'dependencies',
        titleKey: 'ovrenNet.licenseDependencyDocument',
        actionKey: 'thirdPartyLicenses.readLicense',
        format: 'text',
        content: ovrenNetDependencies,
      },
      {
        id: 'windows-adapter',
        titleKey: 'ovrenNet.licenseWindowsDocument',
        actionKey: 'thirdPartyLicenses.readLicense',
        format: 'text',
        content: ovrenNetWindowsLicense,
      },
    ],
  },
];
