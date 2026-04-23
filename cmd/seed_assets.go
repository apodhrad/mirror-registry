package cmd

// seedPlaybookYAML is the seed_mirror_appliance.yml playbook, embedded in
// the binary so that the seed command works independently of the EE version.
const seedPlaybookYAML = `- name: "Seed Mirror Appliance"
  gather_facts: yes
  hosts: all
  tasks:
    - name: Expand variables
      include_role:
        name: mirror_appliance
        tasks_from: expand-vars

    - name: Pre-populate seed image blobs
      include_role:
        name: mirror_appliance
        tasks_from: pre-populate-seed-blobs

    - name: Seed registry images
      include_role:
        name: mirror_appliance
        tasks_from: seed-registry-images
`

// prePoulateTaskYAML is the pre-populate-seed-blobs.yaml role task file.
const prePoulateTaskYAML = `- name: Checking for Seed Images Archive
  local_action: stat path=/runner/seed-images.tar
  register: seed_archive

- name: Create seed images directory
  ansible.builtin.file:
    path: "{{ quay_root }}/seed-images"
    state: directory
    recurse: yes
  when: seed_archive.stat.exists

- name: Copy seed images archive to target
  copy:
    src: /runner/seed-images.tar
    dest: "{{ quay_root }}/seed-images.tar"
  when: seed_archive.stat.exists

- name: Unpack seed images archive
  command: "tar -xf {{ quay_root }}/seed-images.tar -C {{ quay_root }}/seed-images/"
  when: seed_archive.stat.exists

- name: Create Quay Storage named volume for blob pre-population
  containers.podman.podman_volume:
    state: present
    name: "{{ quay_storage }}"
  when: "seed_archive.stat.exists and not quay_storage.startswith('/')"

- name: Get quay storage filesystem path
  shell: |
    if [[ "{{ quay_storage }}" == /* ]]; then
      echo "{{ quay_storage }}"
    else
      podman volume inspect "{{ quay_storage }}" | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['Mountpoint'])"
    fi
  register: storage_path
  when: seed_archive.stat.exists

- name: Pre-populate seed image blobs into Quay storage
  shell: |
    set -e
    quay_root=$(eval echo "{{ quay_root }}")
    storage="{{ storage_path.stdout | trim }}"
    blob_base="${storage}/docker/registry/v2/blobs/sha256"
    mkdir -p "${blob_base}"
    for archive in "${quay_root}/seed-images/"*.tar; do
      echo "Pre-populating blobs from $(basename ${archive})"
      tmpdir=$(mktemp -d -p "${storage}")
      tar -xf "${archive}" -C "${tmpdir}"
      for blob in "${tmpdir}/blobs/sha256/"*; do
        digest=$(basename "${blob}")
        first2="${digest:0:2}"
        dest="${blob_base}/${first2}/${digest}"
        mkdir -p "${dest}"
        if [[ ! -f "${dest}/data" ]]; then
          mv "${blob}" "${dest}/data"
        fi
      done
      rm -rf "${tmpdir}"
    done
  when: seed_archive.stat.exists
`

// seedRegistryTaskYAML is the seed-registry-images.yaml role task file.
const seedRegistryTaskYAML = `- name: Checking for Seed Images Archive
  local_action: stat path=/runner/seed-images.tar
  register: seed_archive

- name: Push seed images to Quay registry
  shell: |
    set -e
    quay_root=$(eval echo "${QUAY_ROOT}")
    while IFS= read -r image; do
      [[ -z "$image" || "$image" =~ ^[[:space:]]*# ]] && continue
      filename=$(echo "$image" | sed 's|[/:@]|_|g').tar
      archive="${quay_root}/seed-images/${filename}"
      image_without_digest="${image%@sha256:*}"
      image_path="${image_without_digest#*/}"
      if [[ "$image_path" == */* ]]; then
        namespace="${image_path%%/*}"
        remainder="${image_path#*/}"
        name=$(basename "$remainder" | sed 's|:.*||')
      else
        namespace="${INIT_USER}"
        name=$(basename "$image_path" | sed 's|:.*||')
      fi
      if [[ "$image_without_digest" == *:* ]]; then
        tag="${image_without_digest##*:}"
      else
        tag="latest"
      fi
      if [[ "$namespace" != "${INIT_USER}" ]]; then
        curl -sk -X POST \
          -H "Authorization: Basic $(printf '%s:%s' "${INIT_USER}" "${INIT_PASSWORD}" | base64 -w 0)" \
          -H "Content-Type: application/json" \
          -d "{\"name\": \"${namespace}\", \"email\": \"${namespace}@quay.local\"}" \
          "https://${QUAY_HOSTNAME}/api/v1/organization/" || true
      fi
      target="docker://${QUAY_HOSTNAME}/${namespace}/${name}:${tag}"
      echo "Pushing ${archive} -> ${target}"
      skopeo copy \
        --dest-tls-verify=false \
        --dest-creds "${INIT_USER}:${INIT_PASSWORD}" \
        "oci-archive:${archive}" \
        "${target}"
    done < "${quay_root}/seed-images/seed-images.txt"
  environment:
    QUAY_ROOT: "{{ quay_root }}"
    INIT_USER: "{{ init_user }}"
    INIT_PASSWORD: "{{ init_password }}"
    QUAY_HOSTNAME: "{{ quay_hostname }}"
  when: seed_archive.stat.exists
`
