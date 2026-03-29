# ARM64 Port Progress

Derived from `ARM64_TEMPLATE_AUDIT.md` and reconciled against local artifacts under `local/local_logs` and `local/test-results`.

Status rules used here:

- `not-started`: no recorded build or full-test artifact was found
- `build-attempted`: build evidence exists, but no finalized image was recorded
- `built`: finalized image recorded, but no completed full-test result was recorded
- `tested`: a completed full-test result was recorded, but the recipe is not yet both built and passing in the available evidence
- `completed`: successful image build and a passing completed full-test result were both recorded

Status totals:

- `not-started`: 42
- `build-attempted`: 68
- `built`: 5
- `tested`: 12
- `completed`: 43
- `total`: 170

| Container | Status | Notes |
|---|---|---|
| `afib1` | `built` | Successful local build log `local/local_logs/build_afib1.log`; ARM64 image `afib1:1.6.0` built and `bet2` plus `/opt/code/python-ismrmrd-server/afib1.py` smoke checks passed |
| `afni` | `build-attempted` | Local build log `local/local_logs/build_afni.log` exists, but it does not record a finalized image |
| `amico` | `completed` | Arm64 image built and full test passed |
| `ants` | `tested` | Full test completed and failed immediately on arm64 launcher exec-format issue |
| `apptainer` | `completed` | Successful local build log `local/local_logs/build_apptainer.log` and passing full-test artifact(s) `local/test-results/apptainer-fulltest.json` |
| `arfiproc` | `built` | Successful local build log `local/local_logs/build_arfiproc.log`; ARM64 image `arfiproc:1.0.0` built and `/opt/code/python-ismrmrd-server/arfiproc.py` smoke check passed |
| `ashs` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `aslprep` | `build-attempted` | Local build log `local/local_logs/build_aslprep.log` exists, but it does not record a finalized image |
| `b0map` | `built` | Successful local build log `local/local_logs/build_b0map.log`; ARM64 image `b0map:1.0.0` built and `bet2` plus `/opt/code/python-ismrmrd-server/b0map.py` smoke checks passed |
| `bart` | `completed` | Successful local build log `local/local_logs/build_bart.log` and passing full-test artifact(s) `local/test-results/bart-fulltest.json` |
| `batchheudiconv` | `completed` | Successful local build log `local/local_logs/build_batchheudiconv.log` and passing full-test artifact(s) `local/test-results/batchheudiconv-fulltest.json` |
| `bidsappaa` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappbaracus` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappbrainsuite` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsapphcppipelines` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappmrtrix3connectome` | `build-attempted` | Local build log `local/local_logs/build_bidsappmrtrix3connectome.log` exists, but it does not record a finalized image |
| `bidsapppymvpa` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidsappspm` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `bidscoin` | `completed` | Successful local build log `local/local_logs/build_bidscoin.log` and passing full-test artifact(s) `local/test-results/bidscoin-fulltest.json` |
| `bidsme` | `completed` | Successful local build log `local/local_logs/build_bidsme.log` and passing full-test artifact(s) `local/test-results/bidsme-fulltest.json` |
| `bidstools` | `built` | Arm64 image built; full-test rerun was started but no completed result was recorded |
| `blastct` | `completed` | Arm64 image built and full test passed |
| `blender` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `braid` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `brainager` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `brainlesion` | `build-attempted` | Build progressed into dependency installation, but no finalized image was recorded |
| `brainlifecli` | `completed` | Successful local build log `local/local_logs/build_brainlifecli.log` and passing full-test artifact(s) `local/test-results/brainlifecli-fulltest.json` |
| `brainstorm` | `build-attempted` | Local build log `local/local_logs/build_brainstorm.log` exists, but it does not record a finalized image |
| `builder` | `completed` | Successful local build log `local/local_logs/build_builder.log` and passing full-test artifact(s) `local/test-results/builder-fulltest.json` |
| `cat12` | `build-attempted` | Local build log `local/local_logs/build_cat12.log` exists, but it does not record a finalized image |
| `cbsb0stats` | `completed` | Successful local build log `local/local_logs/build_cbsb0stats.log` and passing full-test artifact(s) `local/test-results/cbsb0stats-fulltest.json` |
| `civet` | `build-attempted` | Local build log `local/local_logs/build_civet.log` exists, but it does not record a finalized image |
| `clearswi` | `completed` | Successful local build log `local/local_logs/build_clearswi.log` and passing full-test artifact(s) `local/test-results/clearswi-fulltest.json` |
| `clinica` | `build-attempted` | Local build log `local/local_logs/build_clinica.log` exists, but it does not record a finalized image |
| `clinicadl` | `build-attempted` | Local build log `local/local_logs/build_clinicadl.log` exists, but it does not record a finalized image |
| `code` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `condaenvs` | `build-attempted` | Build progressed to upstream source acquisition, but no finalized image was recorded |
| `conn` | `build-attempted` | Local build log `local/local_logs/build_conn.log` exists, but it does not record a finalized image |
| `connectomemapper3` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `connectomeworkbench` | `completed` | Successful local build log `local/local_logs/build_connectomeworkbench.log` and passing full-test artifact(s) `local/test-results/connectomeworkbench-fulltest.json` |
| `convert3d` | `tested` | Arm64 image built, but full test completed with x86_64-only runtime payload failures |
| `cosmomvpa` | `completed` | Successful local build log `local/local_logs/build_cosmomvpa.log` and passing full-test artifact(s) `local/test-results/cosmomvpa-fulltest.json` |
| `cpac` | `build-attempted` | Local build log `local/local_logs/build_cpac.log` exists, but it does not record a finalized image |
| `dafne` | `build-attempted` | Local build log `local/local_logs/build_dafne.log` exists, but it does not record a finalized image |
| `datalad` | `completed` | Successful local build log `local/local_logs/build_datalad.log` and passing full-test artifact(s) `local/test-results/datalad-fulltest.json` |
| `dcm2bids` | `completed` | Successful local build log `local/local_logs/build_dcm2bids.log` and passing full-test artifact(s) `local/test-results/dcm2bids-fulltest.json` |
| `dcm2niix` | `completed` | Successful local build log `local/local_logs/build_dcm2niix.log` and passing full-test artifact(s) `local/test-results/dcm2niix-fulltest.json` |
| `deepisles` | `build-attempted` | Local build log `local/local_logs/build_deepisles.log` exists, but it does not record a finalized image |
| `deeplabcut` | `build-attempted` | Local build log `local/local_logs/build_deeplabcut.log` exists, but it does not record a finalized image |
| `deepretinotopy` | `build-attempted` | Local build log `local/local_logs/build_deepretinotopy.log` exists, but it does not record a finalized image |
| `deepsif` | `build-attempted` | Local build log `local/local_logs/build_deepsif.log` exists, but it does not record a finalized image |
| `delphi` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `dicompare` | `completed` | Successful local build log `local/local_logs/build_dicompare.log` and passing full-test artifact(s) `local/test-results/dicompare-fulltest.json` |
| `dicomtools` | `completed` | Successful local build log `local/local_logs/build_dicomtools.log` and passing full-test artifact(s) `local/test-results/dicomtools-fulltest.json` |
| `diffusiontoolkit` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `dsistudio` | `tested` | Existing image full test completed and failed on arm64 runtime incompatibility |
| `eeglab` | `build-attempted` | Local build log `local/local_logs/build_eeglab.log` exists, but it does not record a finalized image |
| `eharmonize` | `completed` | Arm64 image built and full test passed |
| `elastix` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `esilpd` | `build-attempted` | Local build log `local/local_logs/build_esilpd.log` exists, but it does not record a finalized image |
| `exploreasl` | `build-attempted` | Local build log `local/local_logs/build_exploreasl.log` exists, but it does not record a finalized image |
| `ezbids` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `fastsurfer` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `fieldtrip` | `build-attempted` | Local build log `local/local_logs/build_fieldtrip.log` exists, but it does not record a finalized image |
| `fitlins` | `completed` | Arm64 image built and full test passed |
| `fmriprep` | `build-attempted` | Local build log `local/local_logs/build_fmriprep.log` exists, but it does not record a finalized image |
| `freesurfer` | `build-attempted` | Local build log `local/local_logs/build_freesurfer.log` exists, but it does not record a finalized image |
| `fsl` | `tested` | Existing image full test completed and failed on amd64/arm64 mismatch |
| `fsqc` | `completed` | Arm64 image built and full test passed |
| `gigaconnectome` | `build-attempted` | Local build log `local/local_logs/build_gigaconnectome.log` exists, but it does not record a finalized image |
| `gimp` | `completed` | Successful local build log `local/local_logs/build_gimp.log` and passing full-test artifact(s) `local/test-results/gimp-fulltest.json` |
| `gingerale` | `completed` | Arm64 image built and full test passed |
| `glmsingle` | `completed` | Successful local build log `local/local_logs/build_glmsingle.log`; ARM64 image `glmsingle:1.2` built and passing full-test artifact(s) `local/test-results/glmsingle-fulltest.json` recorded after aligning the suite with the packaged metadata and generated Apptainer labels |
| `gouhfi` | `build-attempted` | Local build log `local/local_logs/build_gouhfi.log` exists, but it does not record a finalized image |
| `halfpipe` | `build-attempted` | Local build log `local/local_logs/build_halfpipe.log` exists, but it does not record a finalized image |
| `hcpasl` | `build-attempted` | Local build log `local/local_logs/build_hcpasl.log` exists, but it does not record a finalized image |
| `hdbet` | `tested` | Local full-test artifact `local/test-results/hdbet-fulltest.json` recorded failures after the arm64 image was built |
| `heudiconv` | `completed` | Successful local build log `local/local_logs/build_heudiconv.log` and passing full-test artifact(s) `local/test-results/heudiconv-fulltest.json` |
| `hmri` | `build-attempted` | Local build log `local/local_logs/build_hmri.log` exists, but it does not record a finalized image |
| `hnncore` | `completed` | Successful local build log `local/local_logs/build_hnncore.log` and passing full-test artifact(s) `local/test-results/hnncore-fulltest.json` |
| `ilastik` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `irkernel` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `itksnap` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `jamovi` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `julia` | `build-attempted` | Local build log `local/local_logs/build_julia.log` exists, but it does not record a finalized image |
| `laynii` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `lesionquantificationtoolkit` | `build-attempted` | Local build log `local/local_logs/build_lesionquantificationtoolkit.log` exists, but it does not record a finalized image |
| `lesymap` | `build-attempted` | Local build log `local/local_logs/build_lesymap.log` exists, but it does not record a finalized image |
| `linda` | `build-attempted` | Local build log `local/local_logs/build_linda.log` exists, but it does not record a finalized image |
| `lipsia` | `completed` | Successful local build log `local/local_logs/build_lipsia.log` and passing full-test artifact(s) `local/test-results/lipsia-fulltest.json` |
| `lqt` | `completed` | Successful local build log `local/local_logs/build_lqt.log` and passing full-test artifact(s) `local/test-results/lqt-fulltest.json` |
| `lstai` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `megnet` | `build-attempted` | Build progressed into package solving, but no finalized image was recorded |
| `metabody` | `build-attempted` | Local build log `local/local_logs/build_metabody.log` exists, but it does not record a finalized image |
| `mgltools` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `micapipe` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `minc` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `mipav` | `build-attempted` | Build attempt reached bundled x86_64 JRE exec-format failure |
| `mitkdiffusion` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `mne` | `completed` | Arm64 image built and full test passed |
| `mricrogl` | `tested` | Image built, but full test completed with bundled dcm2niix arm64 incompatibility |
| `mricron` | `build-attempted` | Local build log `local/local_logs/build_mricron.log` exists, but it does not record a finalized image |
| `mriqc` | `build-attempted` | Local build log `local/local_logs/build_mriqc.log` exists, but it does not record a finalized image |
| `mritools` | `tested` | Existing image full test completed and failed immediately on arm64 launcher incompatibility |
| `mrsimetabolicconnectome` | `build-attempted` | Local build log `local/local_logs/build_mrsimetabolicconnectome.log` exists, but it does not record a finalized image |
| `mrtrix3` | `build-attempted` | Local build log `local/local_logs/build_mrtrix3.log` exists, but it does not record a finalized image |
| `mrtrix3tissue` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `musclemap` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `networkcorrespondancetoolkit` | `build-attempted` | Local build log `local/local_logs/build_networkcorrespondancetoolkit.log` exists, but it does not record a finalized image |
| `neurocommand` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `neurodock` | `build-attempted` | Local build log `local/local_logs/build_neurodock.log` exists, but it does not record a finalized image |
| `nftsim` | `build-attempted` | Local build log `local/local_logs/build_nftsim.log` exists, but it does not record a finalized image |
| `nibabies` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `niftyreg` | `completed` | Arm64 image built and full test passed |
| `nighres` | `build-attempted` | Local build log `local/local_logs/build_nighres.log` exists, but it does not record a finalized image |
| `niimath` | `completed` | Successful local build log `local/local_logs/build_niimath.log` and passing full-test artifact(s) `local/test-results/niimath-fulltest.json` |
| `niistat` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `nipype` | `build-attempted` | Local build log `local/local_logs/build_nipype.log` exists, but it does not record a finalized image |
| `noddi` | `build-attempted` | Local build log `local/local_logs/build_noddi.log` exists, but it does not record a finalized image |
| `openads` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `openreconexample` | `completed` | Successful local build log `local/local_logs/build_openreconexample.log` and passing full-test artifact(s) `local/test-results/openreconexample-fulltest.json` |
| `oshyx` | `build-attempted` | Local build log `local/local_logs/build_oshyx.log` exists, but it does not record a finalized image |
| `osprey` | `build-attempted` | Local build log `local/local_logs/build_osprey.log` exists, but it does not record a finalized image |
| `ospreybids` | `build-attempted` | Local build log `local/local_logs/build_ospreybids.log` exists, but it does not record a finalized image |
| `palm` | `completed` | Successful local build log `local/local_logs/build_palm.log` and passing full-test artifact(s) `local/test-results/palm-fulltest.json` |
| `palmettobug` | `build-attempted` | Local build log `local/local_logs/build_palmettobug.log` exists, but it does not record a finalized image |
| `pcntoolkit` | `completed` | Successful local build log `local/local_logs/build_pcntoolkit.log`; ARM64 image `pcntoolkit:0.35` built and passing full-test artifact `local/test-results/pcntoolkit-fulltest.json` recorded via the shared docker workflow on arm64 |
| `petprep` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `physio` | `build-attempted` | Local build log `local/local_logs/build_physio.log` exists, but it does not record a finalized image |
| `prostatefiducialseg` | `build-attempted` | Local build log `local/local_logs/build_prostatefiducialseg.log` exists, but it does not record a finalized image |
| `pydeface` | `build-attempted` | Local build log `local/local_logs/build_pydeface.log` exists, but it does not record a finalized image |
| `qsiprep` | `build-attempted` | Local build log `local/local_logs/build_qsiprep.log` exists, but it does not record a finalized image |
| `qsirecon` | `build-attempted` | Local build log `local/local_logs/build_qsirecon.log` exists, but it does not record a finalized image |
| `qsmbly` | `completed` | Successful local build log `local/local_logs/build_qsmbly.log` and passing full-test artifact(s) `local/test-results/qsmbly-fulltest.json` |
| `qsmxt` | `build-attempted` | Local build log `local/local_logs/build_qsmxt.log` exists, but it does not record a finalized image |
| `quickshear` | `build-attempted` | Local build log `local/local_logs/build_quickshear.log` exists, but it does not record a finalized image |
| `qupath` | `tested` | Existing image full test completed and failed immediately on arm64 launcher issue |
| `rabies` | `build-attempted` | Local build log `local/local_logs/build_rabies.log` exists, but it does not record a finalized image |
| `radtract` | `completed` | Successful local build log `local/local_logs/build_radtract.log` and passing full-test artifact(s) `local/test-results/radtract-fulltest.json` |
| `romeo` | `tested` | Existing image full test completed and failed immediately on arm64 launcher exec-format issue |
| `root` | `build-attempted` | Local build log `local/local_logs/build_root.log` exists, but it does not record a finalized image |
| `rshrf` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `rstudio` | `build-attempted` | Local build log `local/local_logs/build_rstudio.log` exists, but it does not record a finalized image |
| `samsrfx` | `build-attempted` | Local build log `local/local_logs/build_samsrfx.log` exists, but it does not record a finalized image |
| `segmentator` | `completed` | Arm64 image built and full test passed |
| `sigviewer` | `completed` | Successful local build log `local/local_logs/build_sigviewer.log` and passing full-test artifact(s) `local/test-results/sigviewer-fulltest.json` |
| `slicer` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `slicersalt` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `soopct` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `spant` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `spinalcordtoolbox` | `build-attempted` | Local build log `local/local_logs/build_spinalcordtoolbox.log` exists, but it does not record a finalized image |
| `spm12` | `build-attempted` | Local build log `local/local_logs/build_spm12.log` exists, but it does not record a finalized image |
| `spm25` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `spmpython` | `completed` | Successful local build log `local/local_logs/build_spmpython.log` and passing full-test artifact(s) `local/test-results/spmpython-fulltest.json` |
| `surfice` | `build-attempted` | Local build log `local/local_logs/build_surfice.log` exists, but it does not record a finalized image |
| `syncro` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `template` | `completed` | Successful local build log `local/local_logs/build_template.log` and passing full-test artifact(s) `local/test-results/template-fulltest.json` |
| `terastitcher` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `tinyrange` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `topaz` | `completed` | Successful local build log `local/local_logs/build_topaz.log` and passing full-test artifact(s) `local/test-results/topaz-fulltest.json` |
| `totalsegmentator` | `built` | Successful local build log `local/local_logs/build_totalsegmentator.log`; ARM64 image `totalsegmentator:2.5.0` built and `TotalSegmentator --help` plus `python -c "import torch, totalsegmentator"` smoke checks passed |
| `trackvis` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `tractseg` | `build-attempted` | Local build log `local/local_logs/build_tractseg.log` exists, but it does not record a finalized image |
| `vesselboost` | `build-attempted` | Local build log `local/local_logs/build_vesselboost.log` exists, but it does not record a finalized image |
| `vesselvio` | `build-attempted` | Build progressed into the later PyQt build path, but no finalized image was recorded |
| `vina` | `build-attempted` | Local build log `local/local_logs/build_vina.log` exists, but it does not record a finalized image |
| `voreen` | `not-started` | No recorded arm64 build or full-test activity in ARM64_TEMPLATE_AUDIT.md |
| `workshopdemo` | `completed` | Successful local build log `local/local_logs/build_workshopdemo.log` and passing full-test artifact(s) `local/test-results/workshopdemo-fulltest.json` |
| `xcpd` | `build-attempted` | Local build log `local/local_logs/build_xcpd.log` exists, but it does not record a finalized image |
| `xnat` | `completed` | Successful local build log `local/local_logs/build_xnat.log` and passing full-test artifact(s) `local/test-results/xnat-fulltest.json` |
