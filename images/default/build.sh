#!/bin/sh
set -eu
root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
version=${OUTPOST_VERSION:?OUTPOST_VERSION is required}
arch=${OUTPOST_ARCH:-amd64}
[ "$arch" = amd64 ] || { echo "unsupported architecture: $arch" >&2; exit 1; }
out=${OUTPOST_OUTPUT_DIR:-$root/dist}
engine=${OCI_ENGINE:-docker}
epoch=${SOURCE_DATE_EPOCH:?SOURCE_DATE_EPOCH is required}
mkdir -p "$out"
tag="outpost/default:$version-$arch"
tmp=$(mktemp -d)
container=
cleanup() { [ -z "$container" ] || "$engine" rm -f "$container" >/dev/null 2>&1 || true; [ ! -d "$tmp/root" ] || "$engine" run --rm -v "$tmp/root:/out" "$tag" sh -c 'rm -rf /out/* /out/.[!.]* /out/..?*' >/dev/null 2>&1 || true; rm -rf "$tmp"; }
trap cleanup EXIT INT TERM
if [ "${OUTPOST_NO_CACHE:-0}" = 1 ]; then cache=--no-cache; else cache=; fi
"$engine" build $cache --provenance=false --platform="linux/$arch" --build-arg SOURCE_DATE_EPOCH="$epoch" -t "$tag" "$root/images/default"
container=$("$engine" create "$tag")
"$engine" export "$container" > "$tmp/export.tar"
python3 - "$tmp/export.tar" "$tmp/layer.tar" "$out/outpost-default-$version-$arch.oci.tar" "$version" "$epoch" <<'PY'
import hashlib,json,os,sys,tarfile,tempfile,shutil
source,layer,archive,version,epoch=sys.argv[1:]; epoch=int(epoch)
with tarfile.open(source) as src, tarfile.open(layer,'w',format=tarfile.PAX_FORMAT) as dst:
 members=sorted(src.getmembers(),key=lambda x:x.name)
 for m in members:
  if m.name in ('.dockerenv','run/.containerenv','etc/hostname','etc/hosts','etc/resolv.conf'): continue
  m.mtime=epoch; m.uname=m.gname=''; m.pax_headers={}
  data=src.extractfile(m) if m.isfile() else None
  dst.addfile(m,data)
def digest(data): return hashlib.sha256(data).hexdigest()
layer_data=open(layer,'rb').read(); layer_hash=digest(layer_data)
created='1970-01-01T00:00:00Z'
config=json.dumps({'architecture':'amd64','config':{},'created':created,'history':[{'created':created,'created_by':'outpost default image'}],'os':'linux','rootfs':{'diff_ids':['sha256:'+layer_hash],'type':'layers'}},sort_keys=True,separators=(',',':')).encode(); config_hash=digest(config)
manifest=json.dumps({'config':{'digest':'sha256:'+config_hash,'mediaType':'application/vnd.oci.image.config.v1+json','size':len(config)},'layers':[{'digest':'sha256:'+layer_hash,'mediaType':'application/vnd.oci.image.layer.v1.tar','size':len(layer_data)}],'mediaType':'application/vnd.oci.image.manifest.v1+json','schemaVersion':2},sort_keys=True,separators=(',',':')).encode(); manifest_hash=digest(manifest)
index=json.dumps({'manifests':[{'annotations':{'org.opencontainers.image.ref.name':'outpost/default:'+version+'-amd64'},'digest':'sha256:'+manifest_hash,'mediaType':'application/vnd.oci.image.manifest.v1+json','size':len(manifest)}],'schemaVersion':2},sort_keys=True,separators=(',',':')).encode()
layout=b'{"imageLayoutVersion":"1.0.0"}'
d=tempfile.mkdtemp()
try:
 os.makedirs(d+'/blobs/sha256')
 for name,data in [(layer_hash,layer_data),(config_hash,config),(manifest_hash,manifest)]: open(d+'/blobs/sha256/'+name,'wb').write(data)
 open(d+'/index.json','wb').write(index); open(d+'/oci-layout','wb').write(layout)
 with tarfile.open(archive,'w',format=tarfile.PAX_FORMAT) as out:
  for base,dirs,files in os.walk(d):
   dirs.sort(); files.sort()
   for name in dirs+files:
    p=os.path.join(base,name); info=out.gettarinfo(p,os.path.relpath(p,d)); info.uid=info.gid=0; info.uname=info.gname=''; info.mtime=epoch; info.pax_headers={}; out.addfile(info,open(p,'rb') if info.isfile() else None)
finally: shutil.rmtree(d)
PY
mkdir "$tmp/root"
"$engine" run --rm -i -v "$tmp/root:/out" "$tag" tar -C /out -xf - < "$tmp/layer.tar"
"$engine" run --rm -v "$tmp/root:/out" "$tag" find /out -exec touch -h -d "@$epoch" '{}' +
image="outpost-default-$version-$arch.ext4"
cat > "$tmp/make-ext4" <<'SH'
#!/bin/sh
set -eu
truncate -s "$SIZE" "/work/$IMAGE"
E2FSPROGS_FAKE_TIME="$EPOCH" mkfs.ext4 -q -F -d /work/root -O has_journal,^orphan_file,^metadata_csum_seed,^metadata_csum,^resize_inode -E lazy_itable_init=0,hash_seed=11111111-1111-1111-1111-111111111111 -U 22222222-2222-2222-2222-222222222222 "/work/$IMAGE"
inodes=$(tune2fs -l "/work/$IMAGE" | awk -F: '/^Inode count:/ {gsub(/ /,"",$2); print $2}')
seq 1 "$inodes" | while read -r inode; do
 for field in atime ctime mtime crtime; do printf 'set_inode_field <%s> %s %s\n' "$inode" "$field" "$EPOCH"; done
done > /work/debugfs.commands
for field in wtime lastcheck mkfs_time; do printf 'set_super_value %s %s\n' "$field" "$EPOCH"; done >> /work/debugfs.commands
debugfs -w -f /work/debugfs.commands "/work/$IMAGE" >/dev/null 2>&1
chown "$OWNER" "/work/$IMAGE"
SH
chmod 0755 "$tmp/make-ext4"
"$engine" run --rm -e EPOCH="$epoch" -e SIZE="${OUTPOST_ROOTFS_SIZE:-4G}" -e IMAGE="$image" -e OWNER="$(id -u):$(id -g)" -v "$tmp:/work" "$tag" /work/make-ext4
python3 - "$tmp/$image" "$epoch" <<'PY'
import struct,sys
path,epoch=sys.argv[1:]
with open(path,'r+b') as f:
 for offset in (1024+48,1024+64,1024+264): f.seek(offset); f.write(struct.pack('<I',int(epoch)))
PY
mv "$tmp/$image" "$out/$image"
( cd "$out" && sha256sum "outpost-default-$version-$arch.oci.tar" "$image" > checksums.txt )
