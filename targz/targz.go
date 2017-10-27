package targz

import (
	"io"
	"io/ioutil"
	"errors"
	"path/filepath"
	"compress/gzip"
	"archive/tar"
	"os"
)


//将文件或者目录打成.tar.gz的文件
//src是要打包的文件或者目录
//dest是要生成.tar.gz文件的路径
//failIfExist标识：如果dest文件存在，是否要放弃打包，如果否，则会覆盖已存在的文件
func Tar(src string, dest string, failIfExist bool) (err error) {
	src = filepath.Clean(src)

	if !Exists(src) {
		return errors.New("要打包的文件或者目录不存在："+src)
	}

	if FileExists(dest) {
		if failIfExist { //不覆盖已存在的文件
			return errors.New("目标文件已存在："+dest)
		} else { //覆盖掉已存在的文件
			if err := os.Remove(dest); err != nil {
				return err
			}
		}
	}

	//创建空的目标文件
	fw, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer fw.Close()

	gw := gzip.NewWriter(fw)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer func() {
		//判断tw是否关闭成功，如果失败，可能打包的目标文件不完整
		if er := tw.Close(); er != nil {
			err = er
		}
	}()

	fi, err := os.Stat(src)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		//读取目录下的所有文件
		fis, err := ioutil.ReadDir(src)
		if err != nil {
			return err
		}

		last := len(src)-1
		if src[last] != os.PathSeparator {
			src += string(os.PathSeparator)
		}

		//遍历所有文件
		for _, fi := range fis {
			if fi.IsDir() {
				tarDir(src, fi.Name(), tw, fi)
			} else {
				tarFile(src, fi.Name(), tw, fi)
			}
		}

	} else {
		//获取要打包的文件或者目录的所在位置和名称
		srcBase, srcRelative := filepath.Split(filepath.Clean(src))
		return tarFile(srcBase, srcRelative, tw, fi)
	}

	return nil
}

// 因为要执行遍历操作，所以要单独创建一个函数
func tarDir(srcBase string, srcRelative string, tw *tar.Writer, fi os.FileInfo) (err error) {
	//获取完整路径
	srcFull := srcBase+srcRelative

	//判断目录路径是否带`/`，如果没有，添加上
	last := len(srcRelative) - 1
	if srcRelative[last] != os.PathSeparator {
		srcRelative += string(os.PathSeparator)
	}

	//读取目录下的所有文件
	fis, err := ioutil.ReadDir(srcFull)
	if err != nil {
		return err
	}

	//遍历所有文件
	for _, fi := range fis {
		if fi.IsDir() {
			tarDir(srcBase, srcRelative+fi.Name(), tw, fi)
		} else {
			tarFile(srcBase, srcRelative+fi.Name(), tw, fi)
		}
	}

	if len(srcRelative) > 0 {
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}

		hdr.Name = filepath.ToSlash(srcRelative)

		if err =tw.WriteHeader(hdr); err != nil {
			return err
		}
	}
	return nil
}

// 因为要在 defer 中关闭文件，所以要单独创建一个函数
func tarFile(srcBase string, srcRelative string, tw *tar.Writer, fi os.FileInfo) (err error) {
	//获取完整路径
	srcFull := srcBase+srcRelative

	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(srcRelative)

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	// 打开要打包的文件，准备读取
	fr, err := os.Open(srcFull)
	if err != nil {
		return err
	}
	defer fr.Close()

	if _, err := io.Copy(tw, fr); err != nil {
		return err
	}

	return nil
}

//将.tar.gz的文件解压到dstDir文件夹下
//srcTar是要解压的.tar.gz文件
//dstDir是要解压到的目标文件夹
func UnTar(srcTar string, dstDir string) (err error) {
	srcTar = filepath.FromSlash(srcTar)
	//清理路径字符串
	dstDir = filepath.Clean(dstDir) + string(os.PathSeparator)

	if !Exists(srcTar) {
		return errors.New("要解压的文件不存在："+srcTar)
	}

	//打开要解压的文件
	fr, err := os.Open(srcTar)
	if err != nil {
		return err
	}
	defer fr.Close()

	gr, err := gzip.NewReader(fr)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for hdr, err := tr.Next(); err != io.EOF; hdr, err = tr.Next() {
		if err != nil {
			return err
		}

		//获取文件信息
		fi := hdr.FileInfo()

		//获取绝对路径
		dstDirFull := dstDir + hdr.Name

		if fi.IsDir() {
			//创建目录
			err = os.MkdirAll(dstDirFull, fi.Mode().Perm())
			if err != nil {
				return err
			}
			os.Chmod(dstDirFull, fi.Mode().Perm())
		} else {
			// 创建文件所在的目录
			err = os.MkdirAll(filepath.Dir(dstDirFull), os.ModePerm)
			if err != nil {
				return err
			}
			//将tr中的数据写入到文件中
			if err := unTarFile(dstDirFull, tr); err != nil {
				return err
			}
			os.Chmod(dstDirFull, fi.Mode().Perm())
		}
	}

	return nil
}

// 因为要在 defer 中关闭文件，所以要单独创建一个函数
func unTarFile(dstFile string, tr *tar.Reader) (err error) {
	// 创建空文件，准备写入解包后的数据
	fw, err := os.Create(filepath.FromSlash(dstFile))
	if err != nil {
		return err
	}
	defer fw.Close()

	if _, err := io.Copy(fw, tr); err != nil {
		return err
	}
	return nil
}

//判断文件或者目录是否存在
func Exists(src string) bool {
	_, err := os.Stat(src)
	return err == nil || os.IsExist(err)
}

//判断文件是否存在
func FileExists(name string) bool {
	fi, err := os.Stat(name)
	return (err == nil || os.IsExist(err)) && !fi.IsDir()
}