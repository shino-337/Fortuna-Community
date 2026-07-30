// Minimal JS; lodash is in package.json for SBOM/CVE scan (not used in browser)
document.querySelector('main').addEventListener('click', function () {
  console.log('Static site ready');
});
