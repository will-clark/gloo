import re,sys,os

def get_imports(envoy_path, f):
    try:
        with open(os.path.join(envoy_path,"api/",f)) as data:
            alldata = data.read()
            allimports = re.findall("import \"(.*)\";", alldata)
            imports = set([a for a in allimports])
            for i in allimports:
                imports = imports.union(get_imports(envoy_path, i))
            imports.add(f)
            return imports
    except FileNotFoundError as e:
        print("did not find path", f)
        return set()


def get_sorted_imports(f):
    envoy_path = os.getenv("ENVOYPATH")
    if not envoy_path:
        raise Exception("please set ENVOYPATH to envoy's root folder")
    imports = list(get_imports(envoy_path, f))
    folders = set()
    imports.sort()
    return imports

def import_and_copy(f)
    for i in get_sorted_imports(f):
        path = "api/"+i
        if os.path.exists(path):
            folder = os.path.dirname(i)
            option = 'option go_package = "github.com/solo-io/gloo/projects/gloo/pkg/api/external/'+folder+'";'
            if os.getenv("COPY"):
                f = os.system
            else:
                print("Run these shell:")
                f = lambda x: print(x)
            gopath = os.getenv("GOPATH")
            if not gopath {
                gopath = os.getenv("HOME")+"/go"
            }
            basedir = gopath+"/src/github.com/solo-io/gloo/projects/gloo/api/external/"
            f('mkdir -p ' + basedir + folder)
            dest = basedir +i
            f('cp '+path+' ' + dest)
            f("echo '" + option + "' >> " + dest)
            f("echo 'import \"gogoproto/gogo.proto\";' >> " + dest)
            f("echo 'option (gogoproto.equal_all) = true;' >> " + dest)

def main():
    if len(sys.args) != 2:
        print("please run like so:")
        print("    [COPY=1] [GOPATH=...] ENVOYPATH=... {} path-to-proto-in-envoy".format(sys.args[0]))
        print("for example, this will copy route_components.proto and its dependencies to ~/go/src/github.com/solo-io/gloo/projects/gloo/api/external from ~/sources/envoy/api:")
        print("    COPY=1 GOPATH=~/go ENVOYPATH=~/sources/envoy {} envoy/config/route/v3/route_components.proto".format(sys.args[0]))
        os.abort()
    import_and_copy(sys.args[1])


if __name__ == "__main__":
    main()